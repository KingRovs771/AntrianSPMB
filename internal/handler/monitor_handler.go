package handler

import (
	"AntrianSPMB/internal/models"
	service "AntrianSPMB/internal/services"
	"log"

	"github.com/gofiber/fiber/v2"
)

// MonitorHandler menangani rute-rute untuk Layar Monitor TV dan Live Tracking Mobile
type MonitorHandler struct {
	queueService   service.QueueService
	counterService service.CounterService
}

// NewMonitorHandler adalah konstruktor yang sekarang menerima Dependency Injection dari Service
func NewMonitorHandler(qs service.QueueService, cs service.CounterService) *MonitorHandler {
	return &MonitorHandler{
		queueService:   qs,
		counterService: cs,
	}
}

// ActiveCallData adalah struct bantuan untuk menggabungkan data Loket dan Antrian yang sedang dilayani
type ActiveCallData struct {
	Counter models.Counter
	Queue   *models.Queue
}

// GetActiveCalls mengambil semua loket yang sedang melayani/memanggil (Untuk Monitor TV)
// GET /api/monitor/active-calls
func (h *MonitorHandler) GetActiveCalls(c *fiber.Ctx) error {
	// 1. Ambil semua loket yang statusnya aktif (Buka)
	counters, err := h.counterService.GetActiveCounters()
	if err != nil {
		log.Printf("Error mengambil data loket aktif: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Gagal memuat data loket")
	}

	var activeCalls []ActiveCallData

	// 2. Loop setiap loket, cari apakah ada antrian yang sedang di-handle
	for _, counter := range counters {
		queue, _ := h.counterService.GetCurrentActiveCall(counter.ID)

		// Masukkan ke dalam slice (queue bisa bernilai nil jika loket sedang kosong)
		activeCalls = append(activeCalls, ActiveCallData{
			Counter: counter,
			Queue:   queue,
		})
	}

	// 3. Render partial HTML dan kirimkan datanya
	return c.Render("partials/monitor_active_calls", fiber.Map{
		"ActiveCalls": activeCalls,
	}, "")
}

// GetWaitingList mengambil antrian selanjutnya (Untuk Monitor TV)
// GET /api/monitor/waiting-list
func (h *MonitorHandler) GetWaitingList(c *fiber.Ctx) error {
	// 1. Ambil antrian yang statusnya WAITING untuk masing-masing tahapan ruangan
	infoQueue, _ := h.queueService.GetWaitingListByRoom(models.StepInfoRoom)
	accountQueue, _ := h.queueService.GetWaitingListByRoom(models.StepAccountRoom)
	inputQueue, _ := h.queueService.GetWaitingListByRoom(models.StepInputRoom)

	totalWaiting := len(infoQueue) + len(accountQueue) + len(inputQueue)

	// 2. Render partial untuk daftar tunggu
	return c.Render("partials/monitor_waiting_list", fiber.Map{
		"InfoQueue":    infoQueue,
		"AccountQueue": accountQueue,
		"InputQueue":   inputQueue,
		"TotalWaiting": totalWaiting,
	}, "")
}

// GetTrackStatus mengambil info live tracking untuk 1 spesifik murid (Untuk Layar HP)
// GET /api/track/:ticket_id/status
func (h *MonitorHandler) GetTrackStatus(c *fiber.Ctx) error {
	ticketID := c.Params("ticket_id")
	log.Printf("Update status live tracking untuk tiket: %s\n", ticketID)

	// 1. Cari data tiket berdasarkan ID UUID-nya
	queue, err := h.queueService.GetStatus(ticketID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Tiket antrian tidak ditemukan")
	}

	// 2. Kalkulasi jumlah antrean di depan murid ini
	waitingList, _ := h.queueService.GetWaitingListByRoom(queue.CurrentStep)
	waitingAhead := 0

	for i, w := range waitingList {
		// Jika ID-nya cocok, maka index (i) adalah jumlah orang di depannya
		if w.ID == queue.ID {
			waitingAhead = i
			break
		}
	}

	// 3. Kalkulasi estimasi waktu (Misal rata-rata 3 menit per pelayanan)
	estimatedMins := waitingAhead * 3

	// 4. Render ke partial HTML khusus tracking HP
	return c.Render("partials/track_status", fiber.Map{
		"Queue":         queue,
		"WaitingAhead":  waitingAhead,
		"EstimatedMins": estimatedMins,
	}, "")
}

// SetupMonitorRoutes meregistrasikan semua endpoint terkait monitor ke router utama
func (h *MonitorHandler) SetupMonitorRoutes(router fiber.Router) {
	router.Get("/monitor/active-calls", h.GetActiveCalls)
	router.Get("/monitor/waiting-list", h.GetWaitingList)
	router.Get("/track/:ticket_id/status", h.GetTrackStatus)
}
