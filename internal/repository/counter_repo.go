package repository

import (
	"AntrianSPMB/internal/models"

	"gorm.io/gorm"
)

// Menggunakan alias untuk mempermudah contoh jika belum ada path import riil
// import "spmb-antrian/internal/models"

// CounterRepository mendefinisikan kontrak fungsi untuk interaksi database Counter/Loket
type CounterRepository interface {
	FindByID(id uint) (*models.Counter, error)
	FindAllActive() ([]models.Counter, error)
	GetCurrentActiveQueue(counterID uint) (*models.Queue, error)
	UpdateStatus(counter *models.Counter, isActive bool) error
}

type counterRepo struct {
	db *gorm.DB
}

// NewCounterRepository adalah konstruktor untuk membuat instance repository baru
func NewCounterRepository(db *gorm.DB) CounterRepository {
	return &counterRepo{db: db}
}

// FindByID mencari data loket spesifik berdasarkan ID
func (r *counterRepo) FindByID(id uint) (*models.Counter, error) {
	var counter models.Counter
	// Preload "Staff" akan otomatis mengambil data User (petugas) yang sedang berjaga
	err := r.db.Preload("Staff").Where("id = ?", id).First(&counter).Error
	if err != nil {
		return nil, err
	}
	return &counter, nil
}

// FindAllActive mengambil semua loket yang sedang beroperasi (is_active = true)
func (r *counterRepo) FindAllActive() ([]models.Counter, error) {
	var counters []models.Counter
	err := r.db.Where("is_active = ?", true).Find(&counters).Error
	return counters, err
}

// GetCurrentActiveQueue mencari tiket antrian yang SEDANG DILAYANI (atau dipanggil) oleh loket tertentu
func (r *counterRepo) GetCurrentActiveQueue(counterID uint) (*models.Queue, error) {
	var queue models.Queue

	// Cari antrian di mana counter_id cocok DAN statusnya sedang dipanggil atau diproses
	err := r.db.Where("counter_id = ? AND status IN (?, ?)",
		counterID,
		models.StatusCalling,
		models.StatusProcessing,
	).First(&queue).Error

	if err != nil {
		// Jika tidak ada (error ErrRecordNotFound), berarti loket sedang kosong
		return nil, err
	}

	return &queue, nil
}

// UpdateStatus mengubah status loket (buka/tutup)
func (r *counterRepo) UpdateStatus(counter *models.Counter, isActive bool) error {
	counter.IsActive = isActive
	return r.db.Save(counter).Error
}
