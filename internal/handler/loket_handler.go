package handler

import (
	service "AntrianSPMB/internal/services"
	"AntrianSPMB/pkg/sse"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type LoketHandler struct {
	counterService service.CounterService
	queueService   service.QueueService
	sseManager     *sse.Manager
}

func NewLoketHandler(cs service.CounterService, qs service.QueueService, sm *sse.Manager) *LoketHandler {
	return &LoketHandler{
		counterService: cs,
		queueService:   qs,
		sseManager:     sm,
	}
}

func (h *LoketHandler) GetActiveCall(c *fiber.Ctx) error {
	counterID := c.Params("id")
	// asumsikan parsing counterID string ke uint
	id, _ := strconv.ParseUint(counterID, 10, 32)

	// LOGIKA ASLI:
	queue, err := h.counterService.GetCurrentActiveCall(uint(id))
	if err != nil {
		// Jika tidak ada antrian aktif, render loket kosong
		return c.Render("partials/active_call", fiber.Map{
			"CurrentQueue": nil,
			"CounterID":    counterID,
		}, "")
	}
	// Jika ada, render info murid
	return c.Render("partials/active_call", fiber.Map{
		"CurrentQueue": queue,
		"CounterID":    counterID,
	}, "")

}

func (h *LoketHandler) GetWaitingList(c *fiber.Ctx) error {

	counterID := c.Params("id")
	id, _ := strconv.ParseUint(counterID, 10, 32)
	counter, _ := h.counterService.GetCounterByID(uint(id))
	list, _ := h.queueService.GetWaitingListByRoom(counter.RoomType)

	return c.Render("partials/queue_table", fiber.Map{
		"QueueList": list,
	}, "")
}

func (h *LoketHandler) CallNext(c *fiber.Ctx) error {
	counterID := c.Params("id")
	log.Printf("Loket %s memanggil antrian selanjutnya...\n", counterID)
	id, _ := strconv.ParseUint(counterID, 10, 32)

	counter, _ := h.counterService.GetCounterByID(uint(id))
	queue, err := h.queueService.CallNextCustomer(counter.RoomType, uint(id))
	if err != nil {

	}

	c.Set("HX-Trigger", "queueUpdated")

	return c.Render("partials/active_call", fiber.Map{
		"CurrentQueue": queue,
		"CounterID":    counterID,
	}, "")
}

func (h *LoketHandler) CompleteCall(c *fiber.Ctx) error {
	counterID := c.Params("id")
	log.Printf("Loket %s menyelesaikan pelayanan.\n", counterID)
	id, _ := strconv.ParseUint(counterID, 10, 32)

	queue, _ := h.counterService.GetCurrentActiveCall(uint(id))

	h.queueService.FinishCustomerProcess(queue.ID, queue.CurrentStep)

	return c.Render("partials/active_call", fiber.Map{
		"CurrentQueue": nil,
		"CounterID":    counterID,
	}, "")
}

// RecallTV membunyikan bel dan suara di Monitor TV (Speaker Komputer)
func (h *LoketHandler) RecallTV(c *fiber.Ctx) error {
	counterID := c.Params("id")
	log.Printf("Loket %s memanggil ulang di Monitor TV (Speaker).\n", counterID)
	id, _ := strconv.ParseUint(counterID, 10, 32)

	queue, _ := h.counterService.GetCurrentActiveCall(uint(id))
	if queue != nil {
		h.sseManager.Broadcast("monitor_active", "trigger", queue)
	}

	return c.SendStatus(fiber.StatusOK)
}

// RecallHP mengirim notifikasi ulang ke HP murid secara spesifik
func (h *LoketHandler) RecallHP(c *fiber.Ctx) error {
	counterID := c.Params("id")
	log.Printf("Loket %s memanggil ulang ke HP Murid.\n", counterID)
	id, _ := strconv.ParseUint(counterID, 10, 32)

	queue, _ := h.counterService.GetCurrentActiveCall(uint(id))
	if queue != nil {
		// Broadcast ke topik unik tiket tersebut (HP murid)
		h.sseManager.Broadcast(queue.ID, "status_updated", queue)
	}

	return c.SendStatus(fiber.StatusOK)
}

// GetCounterStats mengembalikan data statistik mini untuk dashboard loket
func (h *LoketHandler) GetCounterStats(c *fiber.Ctx) error {
	counterID := c.Params("id")
	id, _ := strconv.ParseUint(counterID, 10, 32)
	
	counter, _ := h.counterService.GetCounterByID(uint(id))
	waitingCount, _ := h.queueService.CountWaiting(counter.RoomType)
	totalToday, _ := h.queueService.CountTotalToday(counter.RoomType)

	return c.Render("partials/counter_stats", fiber.Map{
		"WaitingCount": waitingCount,
		"TotalToday":   totalToday,
	}, "")
}

// SearchQueue mencari nomor antrian berdasarkan query teks
func (h *LoketHandler) SearchQueue(c *fiber.Ctx) error {
	counterID := c.Params("id")
	query := c.FormValue("search")
	id, _ := strconv.ParseUint(counterID, 10, 32)
	
	counter, _ := h.counterService.GetCounterByID(uint(id))
	list, _ := h.queueService.SearchQueue(counter.RoomType, query)

	return c.Render("partials/queue_table", fiber.Map{
		"QueueList": list,
	}, "")
}

// SkipQueue melewati antrian (misal orangnya sudah pulang)
// POST /api/queue/skip/:ticket_id
func (h *LoketHandler) SkipQueue(c *fiber.Ctx) error {
	ticketID := c.Params("ticket_id")
	log.Printf("Melewati tiket antrian: %s\n", ticketID)

	h.queueService.SkipCustomer(ticketID)

	// Trigger tabel untuk refresh
	c.Set("HX-Trigger", "queueUpdated")
	return c.SendStatus(fiber.StatusOK)
}

// ResetQueues menghapus semua data antrian untuk memulai hari baru
func (h *LoketHandler) ResetQueues(c *fiber.Ctx) error {
	log.Println("🚨 Permintaan reset seluruh antrian diterima!")
	
	err := h.queueService.ResetQueues()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Gagal mereset antrian")
	}

	// Beritahu semua client (Monitor, Dashboard, dll) untuk refresh
	h.sseManager.Broadcast("monitor_active", "trigger", nil)
	c.Set("HX-Trigger", "queueUpdated")

	return c.SendString("Antrian Berhasil Direset")
}

// SetupLoketRoutes meregistrasikan semua endpoint loket ke router
func (h *LoketHandler) SetupLoketRoutes(router fiber.Router) {
	// Semua rute ini harusnya dilindungi dengan middleware otentikasi (JWT/Session)
	router.Get("/counter/:id/current", h.GetActiveCall)
	router.Get("/counter/:id/queue/waiting", h.GetWaitingList)
	router.Get("/counter/:id/stats", h.GetCounterStats)
	router.Post("/counter/:id/queue/search", h.SearchQueue)
	router.Post("/counter/:id/call-next", h.CallNext)
	router.Post("/counter/:id/complete", h.CompleteCall)
	router.Post("/counter/:id/recall-tv", h.RecallTV)
	router.Post("/counter/:id/recall-hp", h.RecallHP)

	router.Post("/queue/skip/:ticket_id", h.SkipQueue)
	router.Post("/queue/reset", h.ResetQueues)
}
