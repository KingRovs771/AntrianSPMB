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

// validateCounterStaff memvalidasi apakah petugas yang login memiliki hak akses untuk counterID ini
func (h *LoketHandler) validateCounterStaff(c *fiber.Ctx, counterID uint) bool {
	roleVal := c.Locals("role")
	if roleVal != nil && roleVal.(string) == "ADMIN" {
		return true
	}

	counter, err := h.counterService.GetCounterByID(counterID)
	if err != nil || counter == nil {
		return false
	}

	userIDVal := c.Locals("user_id")
	if userIDVal == nil {
		return false
	}

	var userID uint
	if f, ok := userIDVal.(float64); ok {
		userID = uint(f)
	} else if u, ok := userIDVal.(uint); ok {
		userID = u
	} else {
		return false
	}

	return counter.StaffID != nil && *counter.StaffID == userID
}

func (h *LoketHandler) GetActiveCall(c *fiber.Ctx) error {
	counterID := c.Params("id")
	id, _ := strconv.ParseUint(counterID, 10, 32)

	if !h.validateCounterStaff(c, uint(id)) {
		return c.Status(fiber.StatusForbidden).SendString(`<div class="bg-red-50 text-red-600 p-4 rounded-xl font-bold">Akses ditolak: Anda tidak ditugaskan di loket ini.</div>`)
	}

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

	if !h.validateCounterStaff(c, uint(id)) {
		return c.Status(fiber.StatusForbidden).SendString("<tr><td colspan='4' class='p-4 text-center text-red-500 font-bold'>Akses Ditolak</td></tr>")
	}

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

	if !h.validateCounterStaff(c, uint(id)) {
		return c.Status(fiber.StatusForbidden).SendString(`<div class="bg-red-50 text-red-600 p-4 rounded-xl font-bold">Akses Ditolak</div>`)
	}

	counter, _ := h.counterService.GetCounterByID(uint(id))
	queue, err := h.queueService.CallNextCustomer(counter.RoomType, uint(id))
	if err != nil {
		log.Printf("Gagal memanggil antrian: %v\n", err)
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

	if !h.validateCounterStaff(c, uint(id)) {
		return c.Status(fiber.StatusForbidden).SendString(`<div class="bg-red-50 text-red-600 p-4 rounded-xl font-bold">Akses Ditolak</div>`)
	}

	queue, _ := h.counterService.GetCurrentActiveCall(uint(id))
	if queue != nil {
		h.queueService.FinishCustomerProcess(queue.ID, queue.CurrentStep)
	}

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

	if !h.validateCounterStaff(c, uint(id)) {
		return c.SendStatus(fiber.StatusForbidden)
	}

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

	if !h.validateCounterStaff(c, uint(id)) {
		return c.SendStatus(fiber.StatusForbidden)
	}

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

	if !h.validateCounterStaff(c, uint(id)) {
		return c.Status(fiber.StatusForbidden).SendString("")
	}

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

	if !h.validateCounterStaff(c, uint(id)) {
		return c.Status(fiber.StatusForbidden).SendString("<tr><td colspan='4' class='p-4 text-center text-red-500 font-bold'>Akses Ditolak</td></tr>")
	}

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

	// Validasi apakah tiket ini miliknya / ruangannya jika dibutuhkan (bisa diskip oleh staff manapun atau divalidasi)
	// Untuk keamanan dasar, biarkan AuthMiddleware membatasi akses ke handler ini secara umum
	h.queueService.SkipCustomer(ticketID)

	// Trigger tabel untuk refresh
	c.Set("HX-Trigger", "queueUpdated")
	return c.SendStatus(fiber.StatusOK)
}

// ResetQueues menghapus semua data antrian untuk memulai hari baru
func (h *LoketHandler) ResetQueues(c *fiber.Ctx) error {
	log.Println("🚨 Permintaan reset seluruh antrian diterima!")

	// Siapa saja petugas yang login berhak melakukan reset (karena tombol reset ada di dashboard petugas)

	err := h.queueService.ResetQueues()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Gagal mereset antrian")
	}

	// Beritahu semua client (Monitor, Dashboard, dll) untuk refresh
	if h.sseManager != nil {
		h.sseManager.Broadcast("monitor_active", "trigger", nil)
		h.sseManager.Broadcast("all", "status_updated", nil)
	}
	c.Set("HX-Trigger", "queueUpdated")

	return c.SendString("Antrian Berhasil Direset")
}

// SetupLoketRoutes meregistrasikan semua endpoint loket ke router
func (h *LoketHandler) SetupLoketRoutes(router fiber.Router) {
	// Semua rute ini dilindungi dengan middleware otentikasi (JWT/Session)
	loketGroup := router.Group("/counter/:id", AuthMiddleware())

	loketGroup.Get("/current", h.GetActiveCall)
	loketGroup.Get("/queue/waiting", h.GetWaitingList)
	loketGroup.Get("/stats", h.GetCounterStats)
	loketGroup.Post("/queue/search", h.SearchQueue)
	loketGroup.Post("/call-next", h.CallNext)
	loketGroup.Post("/complete", h.CompleteCall)
	loketGroup.Post("/recall-tv", h.RecallTV)
	loketGroup.Post("/recall-hp", h.RecallHP)

	router.Post("/queue/skip/:ticket_id", AuthMiddleware(), h.SkipQueue)
	router.Post("/queue/reset", AuthMiddleware(), h.ResetQueues)
}
