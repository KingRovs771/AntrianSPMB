package database

import (
	"AntrianSPMB/internal/models"
	"AntrianSPMB/internal/repository"
	"AntrianSPMB/internal/services"
	"log"

	"gorm.io/gorm"
)

// SeedAll menjalankan seluruh proses seeding data awal
func SeedAll(db *gorm.DB, ur repository.UserRepository, cr repository.CounterRepository, as service.AuthService) {
	log.Println("🚀 Memulai proses seeding data...")

	// 1. Daftar User yang akan dibuat
	users := []models.User{
		{Username: "info1", FullName: "Petugas Informasi 1", Role: models.RoleStaffInfo},
		{Username: "akun1", FullName: "Petugas Akun 1", Role: models.RoleStaffAccount},
		{Username: "akun2", FullName: "Petugas Akun 2", Role: models.RoleStaffAccount},
		{Username: "verif1", FullName: "Petugas Verifikasi 1", Role: models.RoleStaffInput},
		{Username: "input1", FullName: "Petugas Input Data 1", Role: models.RoleStaffInput},
	}

	for _, u := range users {
		existing, _ := ur.FindByUsername(u.Username)
		if existing == nil {
			hashed, _ := as.HashPassword("password123")
			u.Password = hashed
			ur.Create(&u)
			log.Printf("✅ User '%s' berhasil dibuat\n", u.Username)
		}
	}

	// 2. Daftar Loket yang akan dibuat
	uInfo1, _ := ur.FindByUsername("info1")
	uAkun1, _ := ur.FindByUsername("akun1")
	uAkun2, _ := ur.FindByUsername("akun2")
	uVerif1, _ := ur.FindByUsername("verif1")
	uInput1, _ := ur.FindByUsername("input1")

	counters := []models.Counter{
		{ID: 1, Name: "Loket 01 - Informasi", RoomType: models.StepInfoRoom, IsActive: true, StaffID: &uInfo1.ID},
		{ID: 2, Name: "Loket 02 - Akun", RoomType: models.StepAccountRoom, IsActive: true, StaffID: &uAkun1.ID},
		{ID: 3, Name: "Loket 03 - Akun", RoomType: models.StepAccountRoom, IsActive: true, StaffID: &uAkun2.ID},
		{ID: 4, Name: "Loket 04 - Verifikasi", RoomType: models.StepInputRoom, IsActive: true, StaffID: &uVerif1.ID},
		{ID: 5, Name: "Loket 05 - Input Data", RoomType: models.StepInputRoom, IsActive: true, StaffID: &uInput1.ID},
	}

	for _, c := range counters {
		var count int64
		db.Model(&models.Counter{}).Where("id = ?", c.ID).Count(&count)
		if count == 0 {
			db.Create(&c)
			log.Printf("✅ Loket '%s' berhasil dibuat\n", c.Name)
		}
	}

	log.Println("✨ Seeding data selesai!")
}
