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
		{Username: "verif1", FullName: "Petugas Pendaftaran 1", Role: models.RoleStaffInput},
		{Username: "input1", FullName: "Petugas Input Data 1", Role: models.RoleStaffInput},
		{Username: "admin", FullName: "Administrator Utama", Role: models.RoleAdmin},
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

	var staffID1, staffID2, staffID3, staffID4, staffID5 *uint
	if uInfo1 != nil { staffID1 = &uInfo1.ID }
	if uAkun1 != nil { staffID2 = &uAkun1.ID }
	if uAkun2 != nil { staffID3 = &uAkun2.ID }
	if uVerif1 != nil { staffID4 = &uVerif1.ID }
	if uInput1 != nil { staffID5 = &uInput1.ID }

	counters := []models.Counter{
		{ID: 1, Name: "Loket 01 - Informasi", RoomType: models.StepInfoRoom, IsActive: true, StaffID: staffID1},
		{ID: 2, Name: "Loket 02 - Akun", RoomType: models.StepAccountRoom, IsActive: true, StaffID: staffID2},
		{ID: 3, Name: "Loket 03 - Akun", RoomType: models.StepAccountRoom, IsActive: true, StaffID: staffID3},
		{ID: 4, Name: "Loket 04 - Pendaftaran", RoomType: models.StepInputRoom, IsActive: true, StaffID: staffID4},
		{ID: 5, Name: "Loket 05 - Input Data", RoomType: models.StepInputRoom, IsActive: true, StaffID: staffID5},
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
