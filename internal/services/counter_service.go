package service

import (
	"AntrianSPMB/internal/models"
	"AntrianSPMB/internal/repository"
	"errors"
	// Sesuaikan dengan modul di go.mod Anda
	// "spmb-antrian/internal/models"
	// "spmb-antrian/internal/repository"
)

// CounterService mendefinisikan kontrak logika bisnis untuk Meja/Loket
type CounterService interface {
	GetCounterByID(id uint) (*models.Counter, error)
	GetActiveCounters() ([]models.Counter, error)

	// Fungsi vital untuk mengecek apakah loket sedang melayani murid atau kosong
	GetCurrentActiveCall(counterID uint) (*models.Queue, error)

	// Fungsi untuk mengaktifkan/menonaktifkan loket (Buka/Tutup)
	ToggleCounterStatus(counterID uint, isActive bool) error
}

type counterService struct {
	counterRepo repository.CounterRepository
}

// NewCounterService adalah konstruktor untuk CounterService
func NewCounterService(cr repository.CounterRepository) CounterService {
	return &counterService{
		counterRepo: cr,
	}
}

// GetCounterByID mengambil profil loket dan petugas yang sedang berjaga
func (s *counterService) GetCounterByID(id uint) (*models.Counter, error) {
	if id == 0 {
		return nil, errors.New("ID loket tidak valid")
	}
	return s.counterRepo.FindByID(id)
}

// GetActiveCounters mengambil daftar semua loket yang statusnya Buka (IsActive = true)
func (s *counterService) GetActiveCounters() ([]models.Counter, error) {
	return s.counterRepo.FindAllActive()
}

// GetCurrentActiveCall mengambil tiket antrian yang SEDANG dipegang/dilayani oleh loket ini
func (s *counterService) GetCurrentActiveCall(counterID uint) (*models.Queue, error) {
	if counterID == 0 {
		return nil, errors.New("ID loket tidak valid")
	}

	// Memanggil repository.
	// Catatan: Jika loket kosong, fungsi ini biasanya akan mengembalikan error (RecordNotFound dari GORM)
	queue, err := s.counterRepo.GetCurrentActiveQueue(counterID)
	if err != nil {
		return nil, err
	}

	return queue, nil
}

// ToggleCounterStatus mengubah status loket menjadi Buka (true) atau Tutup (false)
func (s *counterService) ToggleCounterStatus(counterID uint, isActive bool) error {
	// Pastikan loketnya ada di database
	counter, err := s.counterRepo.FindByID(counterID)
	if err != nil {
		return errors.New("loket tidak ditemukan")
	}

	// Cegah petugas menutup loket jika masih ada murid yang sedang dilayani
	if !isActive {
		activeCall, _ := s.counterRepo.GetCurrentActiveQueue(counterID)
		if activeCall != nil {
			return errors.New("tidak bisa menutup loket karena masih ada murid yang sedang dilayani")
		}
	}

	return s.counterRepo.UpdateStatus(counter, isActive)
}
