package service

import (
	"AntrianSPMB/internal/models"
	"AntrianSPMB/internal/repository"
	"errors"
)

// QueueService mendefinisikan logika bisnis untuk sistem antrian
type QueueService interface {
	CreateTicket() (*models.Queue, error)
	GetStatus(ticketID string) (*models.Queue, error)
	GetWaitingListByRoom(step models.Step) ([]models.Queue, error)

	// Operasi Loket
	CallNextCustomer(step models.Step, counterID uint) (*models.Queue, error)
	FinishCustomerProcess(ticketID string, currentStep models.Step) error
	SkipCustomer(ticketID string) error

	// Statistik
	CountWaiting(step models.Step) (int64, error)
	CountTotalToday(step models.Step) (int64, error)
	CountTotalAll() (int64, error)
	CountToday() (int64, error)

	SearchQueue(step models.Step, query string) ([]models.Queue, error)

	ResetQueues() error
}

type queueService struct {
	queueRepo repository.QueueRepository
	// Nantinya Anda bisa menambahkan SSEManager di sini untuk trigger event real-time
	// sseManager *sse.Manager
}

// NewQueueService adalah konstruktor untuk QueueService
func NewQueueService(qr repository.QueueRepository) QueueService {
	return &queueService{
		queueRepo: qr,
	}
}

// CreateTicket menangani pembuatan tiket baru saat user memencet Kiosk
func (s *queueService) CreateTicket() (*models.Queue, error) {
	// Panggil repository untuk membuat tiket aman dengan Transaction
	newQueue, err := s.queueRepo.GenerateNewTicket()
	if err != nil {
		return nil, err
	}

	// Opsional: Jika Anda punya SSE manager, tembak event ke dashboard admin
	// s.sseManager.Broadcast("admin_dashboard", "new_ticket", newQueue)

	return newQueue, nil
}

// GetStatus mengambil status tiket berdasarkan ID (dipakai untuk Live Tracking HP)
func (s *queueService) GetStatus(ticketID string) (*models.Queue, error) {
	if ticketID == "" {
		return nil, errors.New("ticket ID tidak boleh kosong")
	}
	return s.queueRepo.FindByID(ticketID)
}

// GetWaitingListByRoom mengambil daftar antrian yang menunggu di ruangan tertentu
func (s *queueService) GetWaitingListByRoom(step models.Step) ([]models.Queue, error) {
	return s.queueRepo.GetWaitingList(step)
}

// CallNextCustomer adalah logika inti saat petugas menekan tombol Panggil
func (s *queueService) CallNextCustomer(step models.Step, counterID uint) (*models.Queue, error) {
	// 1. Ambil nomor antrian tertua yang statusnya WAITING dan kunci untuk loket ini
	queue, err := s.queueRepo.CallNext(step, counterID)
	if err != nil {
		return nil, err // Mengembalikan error jika antrian kosong
	}

	// 2. Tembak event real-time via SSE
	// - Broadcast ke Monitor TV: "Kotak loket 1 berkedip dengan nomor A-024"
	// - Broadcast ke HP murid (menggunakan queue.ID): "Giliran Anda, segera menuju loket!"
	// s.sseManager.Broadcast("monitor_active", "trigger", queue)
	// s.sseManager.Broadcast(queue.ID, "status_updated", queue)

	return queue, nil
}

// FinishCustomerProcess dipanggil saat petugas menekan tombol "Selesai"
func (s *queueService) FinishCustomerProcess(ticketID string, currentStep models.Step) error {
	// 1. Cari antrian berdasarkan ID
	queue, err := s.queueRepo.FindByID(ticketID)
	if err != nil {
		return err
	}

	// 2. Tentukan langkah (ruangan) selanjutnya berdasarkan ruangan saat ini
	var nextStep models.Step
	switch currentStep {
	case models.StepInfoRoom:
		nextStep = models.StepAccountRoom
	case models.StepAccountRoom:
		nextStep = models.StepInputRoom
	case models.StepInputRoom:
		nextStep = models.StepCompleted
	default:
		return errors.New("tahapan ruangan tidak valid")
	}

	// 3. Jika sudah di tahap akhir (Input Room), tutup tiket
	if nextStep == models.StepCompleted {
		err = s.queueRepo.UpdateStatus(queue, models.StatusFinished)
	} else {
		// Jika belum akhir, pindahkan antrian ke ruangan selanjutnya (Handoff)
		err = s.queueRepo.MoveToNextStep(queue, nextStep)
	}

	if err != nil {
		return err
	}

	// 4. Trigger SSE Event agar antrian di layar dashboard loket tujuan otomatis ter-update
	// s.sseManager.Broadcast("queueUpdated", "trigger", nil)

	return nil
}

// SkipCustomer digunakan jika nomor dipanggil berkali-kali tapi murid tidak datang
func (s *queueService) SkipCustomer(ticketID string) error {
	queue, err := s.queueRepo.FindByID(ticketID)
	if err != nil {
		return err
	}

	// Ubah status menjadi SKIPPED
	return s.queueRepo.UpdateStatus(queue, models.StatusSkipped)
}

func (s *queueService) CountWaiting(step models.Step) (int64, error) {
	return s.queueRepo.CountWaiting(step)
}

func (s *queueService) CountTotalToday(step models.Step) (int64, error) {
	return s.queueRepo.CountTotalToday(step)
}

func (s *queueService) CountTotalAll() (int64, error) {
	return s.queueRepo.CountTotalAll()
}

func (s *queueService) CountToday() (int64, error) {
	return s.queueRepo.CountToday()
}

func (s *queueService) SearchQueue(step models.Step, query string) ([]models.Queue, error) {
	return s.queueRepo.SearchQueue(step, query)
}

func (s *queueService) ResetQueues() error {
	return s.queueRepo.ResetAll()
}
