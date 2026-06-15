package handler

import (
	"log"

	"AntrianSPMB/internal/models"
	"AntrianSPMB/internal/services"

	"github.com/gofiber/fiber/v2"
)

// KioskHandler menangani semua rute HTTP yang berkaitan dengan layar Kiosk
type KioskHandler struct {
	// Membutuhkan QueueService untuk menjalankan logika bisnis (membuat antrian)
	queueService service.QueueService
}

// NewKioskHandler adalah konstruktor untuk membuat instance KioskHandler
func NewKioskHandler(qs service.QueueService) *KioskHandler {
	return &KioskHandler{
		queueService: qs,
	}
}

// HandleGenerateTicket merespons ketika tombol "Ambil Antrian" ditekan (POST /api/queue/generate)
func (h *KioskHandler) HandleGenerateTicket(c *fiber.Ctx) error {
	log.Println("Menerima request cetak antrian baru dari Kiosk...")

	// Ambil parameter step dari form request
	stepStr := c.FormValue("step")
	var step models.Step
	switch stepStr {
	case "INFO_ROOM":
		step = models.StepInfoRoom
	case "ACCOUNT_ROOM":
		step = models.StepAccountRoom
	case "INPUT_ROOM":
		step = models.StepInputRoom
	default:
		step = models.StepInfoRoom
	}

	log.Printf("Tipe ruangan yang dipilih: %s\n", step)

	// 1. Panggil Service untuk membuat tiket di Database
	queue, err := h.queueService.CreateTicket(step)
	if err != nil {
		log.Printf("Error membuat tiket: %v\n", err)
		// Kembalikan alert error ringan via HTMX jika database bermasalah
		return c.Status(fiber.StatusInternalServerError).
			SendString(`<div class="bg-red-100 text-red-600 p-4 rounded-xl font-bold">Gagal mengambil antrian, server sibuk.</div>`)
	}

	queueNumber := queue.QueueNumber
	ticketID := queue.ID

	log.Printf("Tiket berhasil dibuat: %s (%s)\n", queueNumber, ticketID)

	// 2. Kembalikan Partial HTML (kiosk_modal.html) untuk disuntikkan oleh HTMX
	// Parameter "" (string kosong) di akhir berarti JANGAN gunakan layout utama (main.html)
	return c.Render("partials/kiosk_modal", fiber.Map{
		"QueueNumber": queueNumber,
		"TicketID":    ticketID,
	}, "")
}

// HandleResetModal merespons ketika user menekan tombol "Selesai/Tutup" (GET /api/kiosk/reset)
func (h *KioskHandler) HandleResetModal(c *fiber.Ctx) error {
	// Karena di HTMX (kiosk.html) kita menset hx-target="#kiosk-modal" dan hx-swap="innerHTML",
	// mengembalikan string kosong ("") akan menyapu bersih isi div tersebut, sehingga modal menghilang.
	return c.SendString("")
}

// HandleGetStatus mengembalikan partial HTML status antrian aktif per ruangan (GET /api/kiosk/status)
func (h *KioskHandler) HandleGetStatus(c *fiber.Ctx) error {
	// 1. Ambil antrian aktif (yang sedang dilayani) untuk masing-masing ruangan
	infoActive, _ := h.queueService.GetActiveQueueByRoom(models.StepInfoRoom)
	accountActive, _ := h.queueService.GetActiveQueueByRoom(models.StepAccountRoom)
	inputActive, _ := h.queueService.GetActiveQueueByRoom(models.StepInputRoom)

	// 2. Hitung jumlah antrian yang sedang menunggu (WAITING)
	infoWaiting, _ := h.queueService.CountWaiting(models.StepInfoRoom)
	accountWaiting, _ := h.queueService.CountWaiting(models.StepAccountRoom)
	inputWaiting, _ := h.queueService.CountWaiting(models.StepInputRoom)

	var infoNoActive string = "Belum Ada"
	if infoActive != nil {
		infoNoActive = infoActive.QueueNumber
	}
	var accountNoActive string = "Belum Ada"
	if accountActive != nil {
		accountNoActive = accountActive.QueueNumber
	}
	var inputNoActive string = "Belum Ada"
	if inputActive != nil {
		inputNoActive = inputActive.QueueNumber
	}

	return c.Render("partials/kiosk_status", fiber.Map{
		"InfoActive":     infoNoActive,
		"AccountActive":  accountNoActive,
		"InputActive":    inputNoActive,
		"InfoWaiting":    infoWaiting,
		"AccountWaiting": accountWaiting,
		"InputWaiting":   inputWaiting,
	}, "")
}

// SetupKioskRoutes adalah fungsi pembantu untuk mendaftarkan rute-rute ini ke Fiber app
func (h *KioskHandler) SetupKioskRoutes(router fiber.Router) {
	// Rute-rute ini akan diakses lewat awalan /api/... (diatur dari main.go)
	router.Post("/queue/generate", h.HandleGenerateTicket)
	router.Get("/kiosk/reset", h.HandleResetModal)
	router.Get("/kiosk/status", h.HandleGetStatus)
}
