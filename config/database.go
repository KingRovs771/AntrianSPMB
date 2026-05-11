package config

import (
	"AntrianSPMB/internal/models"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	// Pastikan mengimpor modul models yang sudah dibuat sebelumnya
	// Sesuaikan "spmb-antrian" dengan nama modul di file go.mod Anda
	// "spmb-antrian/internal/models"
)

// DB adalah variabel global (pointer) yang menyimpan instance koneksi GORM.
// Fungsi lain dapat mengaksesnya melalui config.DB
var DB *gorm.DB

// ConnectDatabase menginisialisasi dan menguji koneksi ke database PostgreSQL
func ConnectDatabase() *gorm.DB {
	// Membaca koneksi dari environment variable (.env)
	// Kita prioritaskan DATABASE_URL yang biasanya berisi string koneksi lengkap (DSN/URL)
	dsn := os.Getenv("DATABASE_URL")
	
	if dsn == "" {
		// Jika DATABASE_URL tidak ada, coba cek DB_DSN sebagai fallback kedua
		dsn = os.Getenv("DB_DSN")
	}

	if dsn == "" {
		// Nilai fallback default jika di .env tidak disetel
		dsn = "host=localhost user=postgres password=rahasia dbname=spmb_antrian port=5432 sslmode=disable TimeZone=Asia/Jakarta"
		log.Println("Peringatan: Variabel DATABASE_URL atau DB_DSN di .env tidak ditemukan, menggunakan nilai default lokal.")
	}

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second, // Peringatan jika query lebih lambat dari 1 detik
			LogLevel:                  logger.Info, // LogLevel Info akan mencetak semua SQL query (berguna untuk debug)
			IgnoreRecordNotFoundError: true,        // Jangan tampilkan error jika mencari data tapi tidak ketemu
			Colorful:                  true,        // Teks berwarna di terminal
		},
	)

	// Membuka koneksi menggunakan driver Postgres
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})

	if err != nil {
		log.Fatalf("❌ Gagal terhubung ke database: %v\nSilakan cek kredensial DB_DSN Anda.", err)
	}

	log.Println("✅ Berhasil terhubung ke database PostgreSQL!")

	// Menyimpan instance koneksi ke variabel global
	DB = db

	return db
}

// MigrateDatabase menjalankan GORM AutoMigrate untuk membuat/memperbarui tabel
// Fungsi ini dipanggil dari main.go setelah ConnectDatabase()
func MigrateDatabase(db *gorm.DB) {
	log.Println("Memulai proses Auto Migration GORM...")

	err := db.AutoMigrate(
		&models.User{},
		&models.Counter{},
		&models.Queue{},
		&models.QueueHistory{},
	)

	if err != nil {
		log.Fatalf("❌ Gagal melakukan migrasi database: %v", err)
	}

	log.Println("✅ Auto Migration selesai. Struktur tabel berhasil diperbarui.")
}
