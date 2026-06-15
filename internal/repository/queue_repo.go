package repository

import (
	"AntrianSPMB/internal/models"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// stepPrefix memetakan setiap Step (ruangan) ke prefix nomor antrian yang unik.
// Ini adalah inti dari revisi: setiap ruangan punya seri nomornya sendiri.
var stepPrefix = map[models.Step]string{
	models.StepInfoRoom:    "I", // Ruang Informasi    -> I-001, I-002, ...
	models.StepAccountRoom: "A", // Ruang Pembuatan Akun -> A-001, A-002, ...
	models.StepInputRoom:   "D", // Ruang Input Data     -> D-001, D-002, ...
}

// getNextQueueNumber adalah helper internal yang men-generate nomor antrian berikutnya
// untuk ruangan (step) tertentu dalam satu sesi transaksi database.
// Fungsi ini HARUS dipanggil dalam sebuah transaction untuk menghindari race condition.
func getNextQueueNumber(tx *gorm.DB, step models.Step) (string, error) {
	prefix, ok := stepPrefix[step]
	if !ok {
		return "", fmt.Errorf("step '%s' tidak memiliki prefix nomor antrian yang terdefinisi", step)
	}

	var lastQueue models.Queue
	today := time.Now().Truncate(24 * time.Hour)

	// Cari nomor terakhir yang dibuat HARI INI untuk ruangan ini (berdasarkan prefix)
	result := tx.Where("queue_number LIKE ? AND created_at >= ?", prefix+"-%", today).
		Order("created_at desc").
		First(&lastQueue)

	var nextNumber int
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Belum ada antrian untuk ruangan ini hari ini, mulai dari 1
			nextNumber = 1
		} else {
			return "", result.Error
		}
	} else {
		// Parsing format "X-NNN" untuk mendapatkan angka terakhir
		var lastNum int
		fmt.Sscanf(lastQueue.QueueNumber, prefix+"-%d", &lastNum)
		nextNumber = lastNum + 1
	}

	return fmt.Sprintf("%s-%03d", prefix, nextNumber), nil
}

// QueueRepository mendefinisikan kontrak fungsi untuk interaksi database Antrian
type QueueRepository interface {
	GenerateNewTicket(step models.Step) (*models.Queue, error)
	FindByID(id string) (*models.Queue, error)
	GetWaitingList(step models.Step) ([]models.Queue, error)
	CallNext(step models.Step, counterID uint) (*models.Queue, error)
	UpdateStatus(queue *models.Queue, status models.Status) error
	MoveToNextStep(queue *models.Queue, nextStep models.Step) error

	// Statistik
	CountWaiting(step models.Step) (int64, error)
	CountTotalToday(step models.Step) (int64, error)
	CountTotalAll() (int64, error)
	CountToday() (int64, error)

	SearchQueue(step models.Step, query string) ([]models.Queue, error)

	ResetAll() error
	GetActiveQueueByRoom(step models.Step) (*models.Queue, error)
}

type queueRepo struct {
	db *gorm.DB
}

// NewQueueRepository adalah konstruktor untuk membuat instance repository baru
func NewQueueRepository(db *gorm.DB) QueueRepository {
	return &queueRepo{db: db}
}

// GenerateNewTicket membuat tiket antrian baru dengan nomor yang berurutan sesuai ruangan (Step)
func (r *queueRepo) GenerateNewTicket(step models.Step) (*models.Queue, error) {
	var newQueue models.Queue

	// Gunakan transaction agar penghitungan nomor urut aman jika banyak yang mencetak bersamaan
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Generate nomor antrian dengan prefix ruangan yang dipilih
		queueNumber, err := getNextQueueNumber(tx, step)
		if err != nil {
			return err
		}

		// Buat objek queue baru dengan CurrentStep sesuai step input
		newQueue = models.Queue{
			QueueNumber: queueNumber,
			CurrentStep: step,
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
// Versi baru: Generate nomor antrian BARU sesuai prefix ruangan tujuan.
// Contoh: murid dari I-003 akan menjadi A-002 saat masuk Ruang Akun.
func (r *queueRepo) MoveToNextStep(queue *models.Queue, nextStep models.Step) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Generate nomor antrian baru sesuai ruangan tujuan
		newQueueNumber, err := getNextQueueNumber(tx, nextStep)
		if err != nil {
			return err
		}

		queue.QueueNumber = newQueueNumber // Nomor antrian berubah sesuai ruangan baru
		queue.CurrentStep = nextStep
		queue.Status = models.StatusWaiting

		// Kosongkan CounterID karena dia sudah lepas dari loket sebelumnya
		// dan sedang menunggu untuk dipanggil loket baru
		queue.CounterID = nil

		if err := tx.Save(queue).Error; err != nil {
			return err
		}

		// Log History dengan nomor dan step baru
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

func (r *queueRepo) CountTotalAll() (int64, error) {
	var count int64
	err := r.db.Model(&models.Queue{}).Count(&count).Error
	return count, err
}

func (r *queueRepo) CountToday() (int64, error) {
	var count int64
	today := time.Now().Truncate(24 * time.Hour)
	err := r.db.Model(&models.Queue{}).Where("created_at >= ?", today).Count(&count).Error
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

func (r *queueRepo) GetActiveQueueByRoom(step models.Step) (*models.Queue, error) {
	var queue models.Queue
	err := r.db.Where("current_step = ? AND status IN ?", step, []models.Status{models.StatusCalling, models.StatusProcessing}).
		Order("updated_at desc").
		First(&queue).Error
	if err != nil {
		return nil, err
	}
	return &queue, nil
}
