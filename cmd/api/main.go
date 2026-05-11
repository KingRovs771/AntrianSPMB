package main

import (
	"AntrianSPMB/config"
	"AntrianSPMB/internal/database"
	"AntrianSPMB/internal/handler"
	"AntrianSPMB/internal/repository"
	"AntrianSPMB/internal/services"
	"AntrianSPMB/pkg/sse"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
	"github.com/joho/godotenv"
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

	// 6. Jalankan Seeding Data (Gunakan Seeder terpusat)
	database.SeedAll(db, userRepo, counterRepo, authService)

	// 7. Inisialisasi Handler
	kioskHandler := handler.NewKioskHandler(queueService)
	loketHandler := handler.NewLoketHandler(counterService, queueService, sseManager)
	monitorHandler := handler.NewMonitorHandler(queueService, counterService)
	authHandler := handler.NewAuthHandler(authService)

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

	// ==========================================
	// 11. ROUTING HALAMAN HTML (WEB VIEWS)
	// ==========================================

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
		return c.Render("pages/monitor_tv", fiber.Map{
			"Title": "Monitor Antrian",
		})
	})

	// Halaman Login Petugas
	app.Get("/login", func(c *fiber.Ctx) error {
		return c.Render("pages/login", fiber.Map{
			"Title": "Login Petugas",
		})
	})

	// Halaman Dashboard Loket (Dilindungi Middleware)
	app.Get("/dashboard/loket", handler.AuthMiddleware(), func(c *fiber.Ctx) error {
		username := c.Cookies("session_user")
		return c.Render("pages/loket", fiber.Map{
			"Title":       "Dashboard Operasional",
			"CounterName": "Ruang Pembuatan Akun",
			"StaffName":   username,
			"CounterID":   1,
		})
	})

	// ==========================================
	// 12. ROUTING API & SSE
	// ==========================================
	api := app.Group("/api")
	kioskHandler.SetupKioskRoutes(api)
	loketHandler.SetupLoketRoutes(api)
	monitorHandler.SetupMonitorRoutes(api)

	// Auth Routes
	api.Post("/auth/login", authHandler.HandleLogin)
	api.Post("/auth/logout", authHandler.HandleLogout)

	// --- Endpoint QR Code ---
	api.Get("/qr/:ticket_id", func(c *fiber.Ctx) error {
		ticketID := c.Params("ticket_id")
		
		// 1. Cek apakah ada base URL yang didefinisikan secara manual di .env (Paling Aman untuk Docker)
		baseURL := os.Getenv("APP_URL")
		
		if baseURL == "" {
			// 2. Jika tidak ada, coba deteksi secara otomatis
			scheme := "http"
			// Cek header X-Forwarded-Proto dari Nginx/Proxy
			if c.Get("X-Forwarded-Proto") == "https" || c.Protocol() == "https" {
				scheme = "https"
			}
			
			host := c.Hostname()
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			
			if host == "localhost" || host == "127.0.0.1" || host == "" {
				if localIP := getLocalIP(); localIP != "" {
					host = localIP
				}
			}
			
			port := os.Getenv("PORT")
			if port == "" {
				port = "3000"
			}
			
			fullHost := fmt.Sprintf("%s:%s", host, port)
			if port == "80" || port == "443" {
				fullHost = host
			}
			baseURL = fmt.Sprintf("%s://%s", scheme, fullHost)
		}
		
		trackURL := fmt.Sprintf("%s/track/%s", baseURL, ticketID)
		qrAPI := fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=%s", trackURL)
		
		return c.Redirect(qrAPI)
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


