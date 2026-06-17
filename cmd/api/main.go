package main

import (
	"AntrianSPMB/config"
	"AntrianSPMB/internal/database"
	"AntrianSPMB/internal/handler"
	"AntrianSPMB/internal/models"
	"AntrianSPMB/internal/repository"
	"AntrianSPMB/internal/services"
	"AntrianSPMB/pkg/sse"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
	"github.com/joho/godotenv"
	"github.com/skip2/go-qrcode"
)

func main() {
	// 1. Muat variabel environment dari file .env
	err := godotenv.Load()
	if err != nil {
		log.Println("Peringatan: File .env tidak ditemukan. Menggunakan default environment.")
	}

	// 2. Inisialisasi Database
	db := config.ConnectDatabase()
	config.MigrateDatabase(db)

	// 3. Inisialisasi SSE Manager
	sseManager := sse.NewManager()

	// 4. Inisialisasi Repository
	queueRepo := repository.NewQueueRepository(db)
	counterRepo := repository.NewCounterRepository(db)
	userRepo := repository.NewUserRepository(db)

	// 5. Inisialisasi Service
	queueService := service.NewQueueService(queueRepo)
	counterService := service.NewCounterService(counterRepo)
	authService := service.NewAuthService(userRepo)
	userService := service.NewUserService(userRepo)

	// 6. Jalankan Seeding Data (Gunakan Seeder terpusat)
	database.SeedAll(db, userRepo, counterRepo, authService)

	// 7. Inisialisasi Handler
	kioskHandler := handler.NewKioskHandler(queueService)
	loketHandler := handler.NewLoketHandler(counterService, queueService, sseManager)
	monitorHandler := handler.NewMonitorHandler(queueService, counterService)
	authHandler := handler.NewAuthHandler(authService)
	adminHandler := handler.NewAdminHandler(userService, queueService)

	// 7. Konfigurasi Template Engine (HTML Render)
	engine := html.New("./views", ".html")
	engine.AddFunc("title", func(s string) string { return s })

	// 8. Inisialisasi Fiber App
	app := fiber.New(fiber.Config{
		Views:       engine,
		ViewsLayout: "layouts/main",
		AppName:     "SPMB Antrian SMP Negeri 1 Sragen",
		// Penting untuk VPS/Nginx: Mengizinkan Fiber membaca header X-Forwarded-*
		ProxyHeader: "X-Forwarded-For",
	})

	// 9. Middleware Global
	app.Use(logger.New())
	app.Use(recover.New())

	// 10. Menyajikan File Statis
	app.Static("/assets", "./public/assets")
	app.Static("/favicon.png", "./public/favicon.png")
	app.Static("/favicon.ico", "./public/favicon.png")

	// ==========================================
	// 11. ROUTING HALAMAN HTML (WEB VIEWS)
	// ==========================================

	// Halaman Utama (Landing Page)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Render("pages/landing", fiber.Map{
			"Title": "Pilih Layanan",
		})
	})

	// Halaman Kiosk
	app.Get("/kiosk", func(c *fiber.Ctx) error {
		return c.Render("pages/kiosk", fiber.Map{
			"Title": "Ambil Antrian",
		})
	})

	// Halaman Live Tracking (Diakses via HP)
	app.Get("/track/:ticket_id", func(c *fiber.Ctx) error {
		ticketID := c.Params("ticket_id")
		queue, err := queueService.GetStatus(ticketID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("Tiket tidak ditemukan")
		}

		return c.Render("pages/monitor", fiber.Map{
			"Title":       "Live Tracking",
			"TicketID":    ticketID,
			"QueueNumber": queue.QueueNumber,
		})
	})

	// Halaman Monitor TV
	app.Get("/monitor", func(c *fiber.Ctx) error {
		room := c.Query("room") // INFO_ROOM, ACCOUNT_ROOM, INPUT_ROOM, atau kosong untuk seluruhnya
		
		var title string
		var isSpecificRoom bool
		var roomLabel string
		var roomClass string
		
		switch room {
		case "INFO_ROOM":
			title = "Monitor Ruang Informasi"
			isSpecificRoom = true
			roomLabel = "Ruang Informasi"
			roomClass = "room-info"
		case "ACCOUNT_ROOM":
			title = "Monitor Pembuatan Akun"
			isSpecificRoom = true
			roomLabel = "Ruang Pembuatan Akun"
			roomClass = "room-account"
		case "INPUT_ROOM":
			title = "Monitor Input Data"
			isSpecificRoom = true
			roomLabel = "Ruang Input & Verifikasi Data"
			roomClass = "room-input"
		default:
			title = "Monitor Antrian Utama"
			isSpecificRoom = false
			roomLabel = "Semua Ruangan"
			roomClass = ""
		}

		return c.Render("pages/monitor_tv", fiber.Map{
			"Title":          title,
			"Room":           room,
			"IsSpecificRoom": isSpecificRoom,
			"RoomLabel":      roomLabel,
			"RoomClass":      roomClass,
		})
	})

	// Halaman Panggilan Suara (Voice Caller Page)
	app.Get("/panggilan", func(c *fiber.Ctx) error {
		room := c.Query("room")
		if room == "" {
			return c.Render("pages/panggilan_select", fiber.Map{
				"Title": "Pilih Ruangan Panggilan",
			})
		}

		var roomLabel, roomClass, queuePrefix string
		switch room {
		case "INFO_ROOM":
			roomLabel, roomClass, queuePrefix = "Ruang Informasi", "bg-blue-600", "I"
		case "ACCOUNT_ROOM":
			roomLabel, roomClass, queuePrefix = "Pembuatan Akun", "bg-violet-600", "A"
		case "INPUT_ROOM":
			roomLabel, roomClass, queuePrefix = "Input Data & Verifikasi", "bg-emerald-600", "D"
		default:
			return c.Redirect("/panggilan")
		}

		return c.Render("pages/panggilan", fiber.Map{
			"Title":       "Panggilan Suara — " + roomLabel,
			"Room":        room,
			"RoomLabel":   roomLabel,
			"RoomClass":   roomClass,
			"QueuePrefix": queuePrefix,
		})
	})

	// Halaman Login Petugas
	app.Get("/login", func(c *fiber.Ctx) error {
		// Jika sudah login (ada cookie jwt), langsung lempar ke dashboard
		if c.Cookies("jwt_token") != "" {
			return c.Redirect("/dashboard/loket")
		}
		return c.Render("pages/login", fiber.Map{
			"Title": "Login Petugas",
		})
	})

	// Halaman Dashboard Loket (Dilindungi Middleware)
	// Handler ini akan menentukan Counter mana yang relevan dengan petugas yang login
	app.Get("/dashboard/loket", handler.AuthMiddleware(), func(c *fiber.Ctx) error {
		username := c.Cookies("session_user")

		counterID := uint(1) // Default fallback jika tidak ada counter khusus

		// Ambil userID dari Locals (yang diset oleh AuthMiddleware)
		userIDVal := c.Locals("user_id")
		if userIDVal != nil {
			var userID uint
			if f, ok := userIDVal.(float64); ok {
				userID = uint(f)
			} else if u, ok := userIDVal.(uint); ok {
				userID = u
			}

			if userID > 0 {
				var foundCounter models.Counter
				// Cari loket (counter) di mana staff_id cocok dengan userID
				if txErr := db.Where("staff_id = ?", userID).First(&foundCounter).Error; txErr == nil {
					counterID = foundCounter.ID
				}
			}
		}

		counter, err := counterService.GetCounterByID(counterID)

		// Default values jika counter tidak ditemukan
		counterName := "Dashboard Loket"
		roomLabel := "Ruang Informasi"
		roomClass := "room-info"
		queuePrefix := "I"

		if err == nil {
			counterName = counter.Name
			roomLabel, roomClass, queuePrefix = getRoomInfo(string(counter.RoomType))
		}

		return c.Render("pages/loket", fiber.Map{
			"Title":       "Dashboard Operasional — " + roomLabel,
			"CounterName": counterName,
			"StaffName":   username,
			"CounterID":   counterID,
			"RoomLabel":   roomLabel,
			"RoomClass":   roomClass,
			"QueuePrefix": queuePrefix,
		})
	})

	// ==========================================
	// 12. ROUTING API & SSE
	// ==========================================
	api := app.Group("/api")
	kioskHandler.SetupKioskRoutes(api)
	loketHandler.SetupLoketRoutes(api)
	monitorHandler.SetupMonitorRoutes(api)
	adminHandler.SetupAdminRoutes(app) // Halaman Admin (Web)
	adminHandler.SetupAdminRoutes(api) // API Admin

	// Auth Routes
	api.Post("/auth/login", authHandler.HandleLogin)
	api.Post("/auth/logout", authHandler.HandleLogout)

	// Endpoint status antrian halaman utama
	api.Get("/landing/status", func(c *fiber.Ctx) error {
		infoActive, _ := queueService.GetActiveQueueByRoom(models.StepInfoRoom)
		accountActive, _ := queueService.GetActiveQueueByRoom(models.StepAccountRoom)
		inputActive, _ := queueService.GetActiveQueueByRoom(models.StepInputRoom)

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

		return c.Render("partials/landing_status", fiber.Map{
			"InfoActive":    infoNoActive,
			"AccountActive": accountNoActive,
			"InputActive":   inputNoActive,
		}, "")
	})

	// Endpoint status panggilan per ruangan
	api.Get("/panggilan/status", func(c *fiber.Ctx) error {
		room := c.Query("room")
		if room == "" {
			return c.Status(fiber.StatusBadRequest).SendString("Ruangan tidak dispesifikasikan")
		}

		var roomLabel, roomClass, queuePrefix string
		var step models.Step
		switch room {
		case "INFO_ROOM":
			step = models.StepInfoRoom
			roomLabel, roomClass, queuePrefix = "Ruang Informasi", "bg-blue-600", "I"
		case "ACCOUNT_ROOM":
			step = models.StepAccountRoom
			roomLabel, roomClass, queuePrefix = "Pembuatan Akun", "bg-violet-600", "A"
		case "INPUT_ROOM":
			step = models.StepInputRoom
			roomLabel, roomClass, queuePrefix = "Input Data & Verifikasi", "bg-emerald-600", "D"
		default:
			return c.Status(fiber.StatusBadRequest).SendString("Ruangan tidak valid")
		}

		// Ambil antrian aktif (yang sedang dilayani) untuk ruangan ini
		active, _ := queueService.GetActiveQueueByRoom(step)
		var activeNo string = "Belum Ada"
		if active != nil {
			activeNo = active.QueueNumber
		}

		// Hitung jumlah antrian yang sedang menunggu (WAITING)
		waiting, _ := queueService.CountWaiting(step)

		return c.Render("partials/panggilan_status", fiber.Map{
			"ActiveNumber": activeNo,
			"WaitingCount": waiting,
			"Room":         room,
			"RoomLabel":    roomLabel,
			"RoomClass":    roomClass,
			"QueuePrefix":  queuePrefix,
		}, "")
	})

	// --- Endpoint QR Code ---
	api.Get("/qr/:ticket_id", func(c *fiber.Ctx) error {
		ticketID := c.Params("ticket_id")
		
		// 1. Cek apakah ada base URL yang didefinisikan secara manual di .env (Paling Aman untuk Docker)
		baseURL := os.Getenv("APP_URL")
		
		if baseURL == "" {
			// 2. Jika tidak ada, deteksi secara otomatis menggunakan BaseURL dari request
			baseURL = c.BaseURL()

			// Jika diakses dari localhost (biasanya oleh browser di mesin server/kiosk),
			// ganti hostname ke IP Lokal agar HP di jaringan yang sama bisa scan & akses.
			if strings.Contains(baseURL, "localhost") || strings.Contains(baseURL, "127.0.0.1") {
				if localIP := getLocalIP(); localIP != "" {
					baseURL = strings.Replace(baseURL, "localhost", localIP, 1)
					baseURL = strings.Replace(baseURL, "127.0.0.1", localIP, 1)
				}
			}
		}
		
		trackURL := fmt.Sprintf("%s/track/%s", baseURL, ticketID)
		
		// Generate QR Code secara lokal tanpa bergantung pada internet/API eksternal
		png, err := qrcode.Encode(trackURL, qrcode.Medium, 256)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Gagal generate QR Code")
		}

		c.Set("Content-Type", "image/png")
		return c.Send(png)
	})

	// --- Endpoint Print Antrian ---
	// Endpoint ini merender halaman khusus cetak yang dioptimalkan untuk thermal printer
	api.Post("/queue/print/:ticket_id", func(c *fiber.Ctx) error {
		ticketID := c.Params("ticket_id")
		_, err := queueService.GetStatus(ticketID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("Tiket tidak ditemukan")
		}

		// Kirim header kustom HTMX agar browser membuka window baru untuk cetak
		// (Halaman khusus ini akan langsung memicu dialog cetak otomatis)
		c.Set("HX-Trigger", fmt.Sprintf(`{"printTicket": "/api/queue/print-view/%s"}`, ticketID))
		return c.SendStatus(fiber.StatusOK)
	})

	// Halaman tampilan cetak (Tanpa Layout Utama)
	app.Get("/api/queue/print-view/:ticket_id", func(c *fiber.Ctx) error {
		ticketID := c.Params("ticket_id")
		queue, _ := queueService.GetStatus(ticketID)

		return c.Render("pages/print_ticket", fiber.Map{
			"Queue": queue,
		}, "") // Gunakan layout kosong ("")
	})

	// Endpoint SSE
	app.Get("/sse/monitor", sseManager.HandleSSE("monitor_active"))
	app.Get("/sse/track/:ticket_id", func(c *fiber.Ctx) error {
		return sseManager.HandleSSE(c.Params("ticket_id"))(c)
	})



	// 13. Jalankan Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	localIP := getLocalIP()
	log.Println("--------------------------------------------------")
	log.Printf("🚀 Server berjalan di:\n")
	log.Printf("   - Lokal:   http://localhost:%s\n", port)
	if localIP != "" {
		log.Printf("   - Jaringan: http://%s:%s\n", localIP, port)
	}
	log.Println("--------------------------------------------------")
	
	log.Fatal(app.Listen(":" + port))
}

// getLocalIP mengembalikan alamat IP lokal utama mesin ini
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// Cek IP yang bukan loopback (127.0.0.1)
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// getRoomInfo memetakan RoomType (Step) ke data UI: label ramah, CSS class, dan prefix nomor antrian.
// Ini adalah sumber kebenaran tunggal untuk tampilan warna dan label per ruangan.
func getRoomInfo(roomType string) (label, cssClass, prefix string) {
	switch roomType {
	case "ACCOUNT_ROOM":
		return "Ruang Pembuatan Akun", "room-account", "A"
	case "INPUT_ROOM":
		return "Ruang Input Data", "room-input", "D"
	default: // INFO_ROOM
		return "Ruang Informasi", "room-info", "I"
	}
}
