package repository

import (
	"AntrianSPMB/internal/models"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	// Sesuaikan dengan nama modul Anda di go.mod
	// "spmb-antrian/internal/models"
)

// Menggunakan alias untuk mempermudah contoh jika belum ada path import riil
// Asumsikan models sudah diimport dengan benar
// import "spmb-antrian/internal/models"

// QueueRepository mendefinisikan kontrak fungsi untuk interaksi database Antrian
type QueueRepository interface {
	GenerateNewTicket() (*models.Queue, error)
	FindByID(id string) (*models.Queue, error)
	GetWaitingList(step models.Step) ([]models.Queue, error)
	CallNext(step models.Step, counterID uint) (*models.Queue, error)
	UpdateStatus(queue *models.Queue, status models.Status) error
	MoveToNextStep(queue *models.Queue, nextStep models.Step) error

	// Statistik
	CountWaiting(step models.Step) (int64, error)
	CountTotalToday(step models.Step) (int64, error)

	SearchQueue(step models.Step, query string) ([]models.Queue, error)

	ResetAll() error
}

type queueRepo struct {
	db *gorm.DB
}

// NewQueueRepository adalah konstruktor untuk membuat instance repository baru
func NewQueueRepository(db *gorm.DB) QueueRepository {
	return &queueRepo{db: db}
}

// GenerateNewTicket membuat tiket antrian baru dengan nomor yang berurutan (A-001, A-002)
func (r *queueRepo) GenerateNewTicket() (*models.Queue, error) {
	var newQueue models.Queue

	// Gunakan transaction agar penghitungan nomor urut aman jika banyak yang mencetak bersamaan
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var lastQueue models.Queue

		// Cari nomor antrian terakhir yang dibuat hari ini (berdasarkan created_at)
		// Memulai dari awal setiap harinya
		today := time.Now().Truncate(24 * time.Hour)
		result := tx.Where("created_at >= ?", today).
			Order("created_at desc").
			First(&lastQueue)

		var nextNumber int
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				// Belum ada antrian hari ini, mulai dari 1
				nextNumber = 1
			} else {
				return result.Error // Error lain dari database
			}
		} else {
			// Skenario nyata: Parsing string "A-024" untuk mendapatkan integer 24 lalu + 1
			// Untuk penyederhanaan pada contoh ini, kita asumsikan menggunakan ID autoincrement
			// atau logic parsing string sederhana.
			// Format "A-%03d" menghasilkan A-001, A-002, dst.
			var lastNum int
			fmt.Sscanf(lastQueue.QueueNumber, "A-%d", &lastNum)
			nextNumber = lastNum + 1
		}

		// Buat objek queue baru
		newQueue = models.Queue{
			QueueNumber: fmt.Sprintf("A-%03d", nextNumber),
			CurrentStep: models.StepInfoRoom, // Selalu mulai dari Ruang Informasi
			Status:      models.StatusWaiting,
		}

		// Simpan ke database
		if err := tx.Create(&newQueue).Error; err != nil {
			return err
		}

		return nil
	})

	return &newQueue, err
}

// FindByID mencari data antrian spesifik berdasarkan UUID
func (r *queueRepo) FindByID(id string) (*models.Queue, error) {
	var queue models.Queue
	err := r.db.Preload("Counter").Where("id = ?", id).First(&queue).Error
	if err != nil {
		return nil, err
	}
	return &queue, nil
}

// GetWaitingList mengambil semua antrian yang sedang berstatus WAITING pada ruangan (step) tertentu
func (r *queueRepo) GetWaitingList(step models.Step) ([]models.Queue, error) {
	var queues []models.Queue
	err := r.db.Where("current_step = ? AND status = ?", step, models.StatusWaiting).
		Order("created_at asc"). // Yang datang duluan (FIFO) berada di atas
		Find(&queues).Error

	return queues, err
}

// CallNext adalah fungsi paling vital.
// Mengambil nomor urut terlama yang statusnya WAITING, dan menguncinya untuk loket tertentu.
func (r *queueRepo) CallNext(step models.Step, counterID uint) (*models.Queue, error) {
	var queue models.Queue

	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Clause("FOR UPDATE") sangat krusial di sini!
		// Ini menginstruksikan PostgreSQL untuk mengunci baris (Row Locking) yang sedang dibaca ini.
		// Jika Loket 1 dan Loket 2 menekan tombol panggil di milidetik yang sama persis,
		// Loket 2 akan dipaksa menunggu sampai Loket 1 selesai melakukan update, sehingga tidak ada nomor ganda.
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("current_step = ? AND status = ?", step, models.StatusWaiting).
			Order("created_at asc").
			First(&queue).Error

		if err != nil {
			return err // Kemungkinan gorm.ErrRecordNotFound (antrian kosong)
		}

		// Ubah status tiket yang didapat
		queue.Status = models.StatusCalling
		queue.CounterID = &counterID

		// Simpan perubahan
		if err := tx.Save(&queue).Error; err != nil {
			return err
		}

		// (Opsional) Catat ke tabel History Audit Log
		history := models.QueueHistory{
			QueueID:   queue.ID,
			Step:      step,
			Status:    models.StatusCalling,
			CounterID: &counterID,
		}
		if err := tx.Create(&history).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &queue, nil
}

// UpdateStatus digunakan untuk mengubah CALLING -> PROCESSING atau SKIPPED
func (r *queueRepo) UpdateStatus(queue *models.Queue, status models.Status) error {
	queue.Status = status
	return r.db.Save(queue).Error
}

// MoveToNextStep (Handoff) dipanggil saat petugas menekan tombol "Selesai".
// Ini akan memindahkan murid ke ruangan (step) berikutnya dan mengembalikan status ke WAITING.
func (r *queueRepo) MoveToNextStep(queue *models.Queue, nextStep models.Step) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		queue.CurrentStep = nextStep
		queue.Status = models.StatusWaiting

		// Kosongkan CounterID karena dia sudah lepas dari loket sebelumnya
		// dan sedang menunggu untuk dipanggil loket baru
		queue.CounterID = nil

		if err := tx.Save(queue).Error; err != nil {
			return err
		}

		// Log History
		history := models.QueueHistory{
			QueueID: queue.ID,
			Step:    nextStep,
			Status:  models.StatusWaiting,
		}
		return tx.Create(&history).Error
	})
}

// CountWaiting menghitung jumlah antrian yang sedang menunggu di step tertentu
func (r *queueRepo) CountWaiting(step models.Step) (int64, error) {
	var count int64
	err := r.db.Model(&models.Queue{}).
		Where("current_step = ? AND status = ?", step, models.StatusWaiting).
		Count(&count).Error
	return count, err
}

// CountTotalToday menghitung total seluruh antrian (semua status) di step tertentu hari ini
func (r *queueRepo) CountTotalToday(step models.Step) (int64, error) {
	var count int64
	today := time.Now().Truncate(24 * time.Hour)
	err := r.db.Model(&models.Queue{}).
		Where("current_step = ? AND created_at >= ?", step, today).
		Count(&count).Error
	return count, err
}

func (r *queueRepo) SearchQueue(step models.Step, query string) ([]models.Queue, error) {
	var queues []models.Queue
	err := r.db.Where("current_step = ? AND queue_number ILIKE ?", step, "%"+query+"%").
		Order("created_at asc").
		Find(&queues).Error
	return queues, err
}

func (r *queueRepo) ResetAll() error {
	// Alih-alih menghapus (Delete), kita ubah status antrian yang masih aktif 
	// (WAITING, CALLING, PROCESSING) menjadi SKIPPED agar dashboard bersih 
	// tapi data tetap ada di database untuk keperluan rekapitulasi/laporan.
	return r.db.Model(&models.Queue{}).
		Where("status IN ?", []models.Status{models.StatusWaiting, models.StatusCalling, models.StatusProcessing}).
		Update("status", models.StatusSkipped).Error
}
