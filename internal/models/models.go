package models

import (
	"time"

	"gorm.io/gorm"
)

// =========================================================
// Tipe Data Kustom (Enum)
// =========================================================

// Step mendefinisikan tahapan perjalanan (journey) pendaftaran murid
type Step string

const (
	StepInfoRoom    Step = "INFO_ROOM"    // Tahap 1: Ruang Informasi
	StepAccountRoom Step = "ACCOUNT_ROOM" // Tahap 2: Ruang Pembuatan Akun
	StepInputRoom   Step = "INPUT_ROOM"   // Tahap 3: Ruang Input & Verifikasi Data
	StepCompleted   Step = "COMPLETED"    // Selesai seluruh proses
)

// Status mendefinisikan status nomor antrian pada satu tahapan (Step)
type Status string

const (
	StatusWaiting    Status = "WAITING"    // Menunggu dipanggil di ruang tunggu
	StatusCalling    Status = "CALLING"    // Sedang dipanggil (Blinking di TV/HP)
	StatusProcessing Status = "PROCESSING" // Sedang duduk berhadapan dengan petugas loket
	StatusFinished   Status = "FINISHED"   // Urusan di loket saat ini sudah selesai
	StatusSkipped    Status = "SKIPPED"    // Dipanggil tapi orangnya tidak hadir
)

// Role mendefinisikan tingkat akses untuk sistem login
type Role string

const (
	RoleAdmin        Role = "ADMIN"
	RoleStaffInfo    Role = "STAFF_INFO"
	RoleStaffAccount Role = "STAFF_ACCOUNT"
	RoleStaffInput   Role = "STAFF_INPUT"
)

// =========================================================
// Definisi Struct Tabel (GORM Models)
// =========================================================

// Queue merepresentasikan satu nomor antrian (Single Ticket)
type Queue struct {
	// ID Menggunakan UUID v4 untuk URL Tracking yang aman (tidak bisa ditebak)
	ID          string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	QueueNumber string `gorm:"type:varchar(10);uniqueIndex;not null" json:"queue_number"` // Contoh: A-024
	CurrentStep Step   `gorm:"type:varchar(20);not null" json:"current_step"`
	Status      Status `gorm:"type:varchar(20);not null;default:'WAITING'" json:"status"`

	// Relasi ke Loket (Counter).
	// Menggunakan pointer (*uint) karena bisa bernilai NULL saat tiket baru dibuat (belum dipanggil)
	CounterID *uint   `json:"counter_id"`
	Counter   Counter `gorm:"foreignKey:CounterID" json:"-"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Counter merepresentasikan Meja/Loket tempat petugas berjaga
type Counter struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `gorm:"type:varchar(50);not null" json:"name"`      // Contoh: "Loket 1 Informasi"
	RoomType Step   `gorm:"type:varchar(20);not null" json:"room_type"` // Loket ini beroperasi untuk tahap apa?
	IsActive bool   `gorm:"default:true" json:"is_active"`              // Apakah loket sedang buka/tutup?

	// Relasi ke Petugas (User) yang sedang login/berjaga di loket ini
	StaffID *uint `json:"staff_id"`
	Staff   User  `gorm:"foreignKey:StaffID" json:"-"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// User merepresentasikan Petugas atau Admin yang memiliki akses login
type User struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Username string `gorm:"type:varchar(50);uniqueIndex;not null" json:"username"`
	Password string `gorm:"type:text;not null" json:"-"` // Disembunyikan dari JSON response untuk keamanan
	FullName string `gorm:"type:varchar(100);not null" json:"full_name"`
	Role     Role   `gorm:"type:varchar(20);not null" json:"role"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// QueueHistory adalah tabel log audit untuk melacak pergerakan waktu
// (Berapa lama murid menunggu di INFO_ROOM sebelum dipanggil ke ACCOUNT_ROOM)
type QueueHistory struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	QueueID string `gorm:"type:uuid;not null;index" json:"queue_id"`
	Queue   Queue  `gorm:"foreignKey:QueueID" json:"-"`

	Step   Step   `gorm:"type:varchar(20);not null" json:"step"`
	Status Status `gorm:"type:varchar(20);not null" json:"status"`

	// Relasi Opsional: Loket mana yang melakukan perubahan status ini
	CounterID *uint `json:"counter_id"`

	CreatedAt time.Time `json:"created_at"` // Timestamp perubahan status terjadi
}
