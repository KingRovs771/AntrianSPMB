🎓 Sistem Informasi Antrian Terpadu - SPMB SMP Negeri 1 Sragen

Sistem antrian real-time berbasis Single Ticket System (Satu Tiket Terintegrasi) yang dirancang khusus untuk memfasilitasi alur Seleksi Penerimaan Murid Baru (SPMB).

Dibangun dengan fokus pada kecepatan, efisiensi sumber daya (low-latency), dan kemudahan operasional menggunakan arsitektur Server-Side Rendering (SSR) modern.

🚀 Tech Stack Utama

Aplikasi ini dikembangkan dengan teknologi yang menjamin performa tinggi dan penggunaan resource server yang minimal:

Backend: Go (Golang) v1.21+

Web Framework: Go Fiber v2 (Berperforma tinggi, Express-like)

ORM: GORM (Mendukung migrasi otomatis)

Database: PostgreSQL (Direkomendasikan untuk produksi) / MySQL / SQLite (untuk tahap pengembangan)

Frontend (Interactivity): HTMX (Manipulasi DOM dinamis tanpa JavaScript berat)

Real-time Protocol: Server-Sent Events (SSE)

Styling: Tailwind CSS (via CDN di development, minified di production)

🏗️ Alur Sistem (Single Ticket Journey)

Aplikasi ini mengatur perjalanan calon murid melalui satu nomor urut yang persisten di berbagai tahapan:

Pintu Masuk (Kiosk): Calon murid mencetak tiket atau scan QR Code. Status: INFO_ROOM.

Tahap 1 - Ruang Informasi: Petugas memberikan pengarahan. Setelah selesai, tiket dilempar secara otomatis ke tahap selanjutnya.

Tahap 2 - Ruang Pembuatan Akun: Murid dipanggil melalui Monitor TV atau Notifikasi HP. Proses pembuatan akun selesai.

Tahap 3 - Ruang Input Data: Murid menyerahkan berkas fisik. Verifikasi selesai, tiket ditutup (Status: COMPLETED).

📂 Struktur Direktori Proyek

Proyek ini mengadopsi pola Clean Architecture untuk kemudahan maintenance:

spmb-antrian/
├── cmd/api/          # Entry point (main.go)
├── config/           # Setup koneksi Database & Environment
├── internal/         # Kode inti (Models, Repository, Service, Handler)
├── pkg/              # Modul independen (SSE Manager, Utils)
├── public/           # File statis (Gambar, CSS tambahan)
├── views/            # Template HTML (Layouts, Pages, HTMX Partials)
└── .env              # (Git Ignored) Variabel konfigurasi server


⚙️ Panduan Menjalankan di Lokal (Development)

Persyaratan Awal (Prerequisites)

Pastikan Anda telah menginstal alat-alat berikut di komputer Anda:

Go (minimal versi 1.21)

Database PostgreSQL atau MySQL.

Langkah-langkah Menjalankan:

Clone Repository:

git clone [https://github.com/organisasi-anda/spmb-antrian.git](https://github.com/organisasi-anda/spmb-antrian.git)
cd spmb-antrian


Instal Dependensi Go:

go mod tidy


Konfigurasi Environment:
Salin file contoh konfigurasi dan sesuaikan nilai kredensial database Anda.

cp .env.example .env


Contoh isi .env:

PORT=3000
DB_DSN="host=localhost user=postgres password=rahasia dbname=spmb_antrian port=5432 sslmode=disable TimeZone=Asia/Jakarta"


Jalankan Aplikasi:
Pada saat pertama kali dijalankan, GORM akan otomatis membuatkan tabel-tabel (AutoMigrate).

go run cmd/api/main.go


Atau jika menggunakan alat bantu live-reload seperti Air:

air


Akses Aplikasi:

Kiosk Utama: http://localhost:3000/kiosk

Layar Monitor TV: http://localhost:3000/monitor

Dashboard Petugas Loket: http://localhost:3000/login

🚢 Panduan Deployment (VPS Ubuntu/Debian)

Aplikasi Golang sangat mudah di-deploy karena akan di-compile menjadi satu file binary yang berdiri sendiri (standalone executable), tanpa perlu menginstal runtime Go di peladen (server) produksi.

1. Build File Binary (Lakukan di mesin lokal/pengembang)

Lakukan build khusus untuk arsitektur Linux. Buka terminal proyek lokal Anda dan jalankan:

GOOS=linux GOARCH=amd64 go build -o spmb-app cmd/api/main.go


Ini akan menghasilkan satu file bernama spmb-app.

2. Persiapan Server Produksi (VPS)

Di server VPS Anda (yang sudah terinstal PostgreSQL dan Nginx):

Buat folder untuk aplikasi, contoh: /var/www/spmb-antrian.

Unggah file spmb-app (dari langkah 1) ke folder tersebut.

Unggah juga folder views/, folder public/, dan file .env ke direktori yang sama (agar aplikasi bisa menemukan file HTML dan statis).

Struktur di dalam folder VPS Anda harus terlihat seperti ini:

/var/www/spmb-antrian/
├── spmb-app      (File binary)
├── .env          (Kredensial DB Production)
├── views/        (Folder template HTML)
└── public/       (Folder file statis)


3. Setup Systemd Service (Agar jalan di background & auto-restart)

Buat file service Linux agar aplikasi bisa berjalan terus menerus.

sudo nano /etc/systemd/system/spmb-antrian.service


Isi dengan konfigurasi berikut:

[Unit]
Description=SPMB Antrian Go Fiber Service
After=network.target

[Service]
User=root
WorkingDirectory=/var/www/spmb-antrian
ExecStart=/var/www/spmb-antrian/spmb-app
Restart=always

[Install]
WantedBy=multi-user.target


Aktifkan dan jalankan service tersebut:

sudo chmod +x /var/www/spmb-antrian/spmb-app
sudo systemctl daemon-reload
sudo systemctl enable spmb-antrian
sudo systemctl start spmb-antrian


4. Setup Nginx (Reverse Proxy)

Konfigurasikan Nginx agar meneruskan permintaan dari domain (misal: antrian.sman1sragen.sch.id) ke port lokal Go Fiber (misal: port 3000). Penting: Pastikan konfigurasi mendukung Server-Sent Events dengan menonaktifkan buffering.

Buat blok server Nginx:

sudo nano /etc/nginx/sites-available/spmb-antrian


Isi dengan:

server {
listen 80;
server_name antrian.sman1sragen.sch.id;

    location / {
        proxy_pass [http://127.0.0.1:3000](http://127.0.0.1:3000);
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        
        # Pengaturan PENTING untuk Server-Sent Events (SSE) / Realtime HTMX
        proxy_set_header Connection '';
        proxy_http_version 1.1;
        chunked_transfer_encoding off;
        proxy_buffering off;
        proxy_cache off;
    }
}


Aktifkan dan muat ulang Nginx:

sudo ln -s /etc/nginx/sites-available/spmb-antrian /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx


Selesai! Aplikasi antrian Anda kini telah mengudara dan siap menangani pendaftaran murid baru.