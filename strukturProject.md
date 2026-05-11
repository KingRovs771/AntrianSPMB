Struktur Proyek Sistem Antrian SPMB (Go Fiber + HTMX)

spmb-antrian/
├── cmd/
│   └── api/
│       └── main.go         # Entry point aplikasi (Inisialisasi Fiber, DB, Routes)
├── config/
│   └── database.go         # Konfigurasi koneksi GORM (PostgreSQL/MySQL)
├── internal/               # Logika inti aplikasi (Tidak bisa di-import proyek luar)
│   ├── models/
│   │   └── models.go       # Structs database GORM (yang baru saja kita buat)
│   ├── repository/         # Interaksi langsung dengan database (Query)
│   │   ├── queue_repo.go
│   │   └── counter_repo.go
│   ├── service/            # Logika bisnis (Validasi, aturan state antrian)
│   │   ├── queue_service.go
│   │   └── counter_service.go
│   └── handler/            # Mengatur Request HTTP dan Response (JSON atau HTML/HTMX)
│       ├── kiosk_handler.go
│       ├── loket_handler.go
│       └── monitor_handler.go
├── pkg/                    # Kode utilitas yang bisa dipakai ulang (Bebas di-import)
│   ├── utils/
│   │   └── response.go     # Helper untuk format response JSON yang konsisten
│   └── sse/
│       └── manager.go      # Logika Server-Sent Events untuk broadcast antrian real-time
├── public/                 # File statis publik
│   ├── css/
│   ├── js/
│   └── assets/             # Logo, suara panggilan (bell.mp3)
├── views/                  # Template HTML (Fiber Template Engine - html/template)
│   ├── layouts/
│   │   └── main.html
│   ├── pages/
│   │   ├── kiosk.html
│   │   ├── monitor.html
│   │   └── loket.html
│   └── partials/           # Potongan HTML yang dikembalikan untuk HTMX
│       ├── kiosk_modal.html
│       ├── queue_table.html
│       └── active_call.html
├── .env                    # Variabel lingkungan (DB_DSN, PORT, dsb)
├── go.mod                  # Daftar dependensi Go
└── go.sum
