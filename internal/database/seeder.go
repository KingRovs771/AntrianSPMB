package database

import (
	"AntrianSPMB/internal/models"
	"AntrianSPMB/internal/repository"
	"AntrianSPMB/internal/services"
	"fmt"
	"log"

	"gorm.io/gorm"
)

// SeedAll menjalankan seluruh proses seeding data awal
func SeedAll(db *gorm.DB, ur repository.UserRepository, cr repository.CounterRepository, as service.AuthService) {
	log.Println("🚀 Memulai proses seeding data...")

	// 1. Daftar User yang akan dibuat
	users := []models.User{
		{Username: "info1", FullName: "Loket 1 Informasi", Role: models.RoleStaffInfo},
		{Username: "loket1akun", FullName: "Loket 1 Akun", Role: models.RoleStaffAccount},
		{Username: "loket2akun", FullName: "Loket 2 Akun", Role: models.RoleStaffAccount},
		{Username: "loket3akun", FullName: "Loket 3 Akun", Role: models.RoleStaffAccount},
		{Username: "loket4akun", FullName: "Loket 4 Akun", Role: models.RoleStaffAccount},
		{Username: "loket5akun", FullName: "Loket 5 Akun", Role: models.RoleStaffAccount},
		{Username: "loket6akun", FullName: "Loket 6 Akun", Role: models.RoleStaffAccount},
		{Username: "spensa_1", FullName: "Loket 1 Pendaftaran", Role: models.RoleStaffInput},
		{Username: "spensa_2", FullName: "Loket 2 Pendaftaran", Role: models.RoleStaffInput},
		{Username: "spensa_3", FullName: "Loket 3 Pendaftaran", Role: models.RoleStaffInput},
		{Username: "spensa_4", FullName: "Loket 4 Pendaftaran", Role: models.RoleStaffInput},
		{Username: "spensa_5", FullName: "Loket 5 Pendaftaran", Role: models.RoleStaffInput},
		{Username: "spensa_6", FullName: "Loket 6 Pendaftaran", Role: models.RoleStaffInput},
		{Username: "admin", FullName: "Administrator Utama", Role: models.RoleAdmin},
	}

	for _, u := range users {
		existing, _ := ur.FindByUsername(u.Username)
		hashed, _ := as.HashPassword("spensa162")
		u.Password = hashed
		if existing == nil {
			ur.Create(&u)
			log.Printf("✅ User '%s' berhasil dibuat\n", u.Username)
		} else {
			// Update password dan data untuk user yang sudah ada
			existing.Password = hashed
			existing.FullName = u.FullName
			existing.Role = u.Role
			db.Save(existing)
			log.Printf("✅ User '%s' berhasil diperbarui\n", u.Username)
		}
	}

	// 2. Daftar Loket yang akan dibuat
	var counters []models.Counter

	// Loket 1 Informasi
	var infoStaff *uint
	if uInfo, err := ur.FindByUsername("info1"); err == nil && uInfo != nil {
		infoStaff = &uInfo.ID
	}
	counters = append(counters, models.Counter{
		ID: 1, Name: "Loket 01 - Informasi", RoomType: models.StepInfoRoom, IsActive: true, StaffID: infoStaff,
	})

	// Loket Akun 1..6 (IDs: 2..7)
	for i := 1; i <= 6; i++ {
		username := fmt.Sprintf("loket%dakun", i)
		var staffID *uint
		if u, err := ur.FindByUsername(username); err == nil && u != nil {
			staffID = &u.ID
		}
		counters = append(counters, models.Counter{
			ID:       uint(i + 1),
			Name:     fmt.Sprintf("Loket %02d - Pembuatan Akun", i + 1),
			RoomType: models.StepAccountRoom,
			IsActive: true,
			StaffID:  staffID,
		})
	}

	// Loket Pendaftaran 1..6 (IDs: 8..13)
	for i := 1; i <= 6; i++ {
		username := fmt.Sprintf("spensa_%d", i)
		var staffID *uint
		if u, err := ur.FindByUsername(username); err == nil && u != nil {
			staffID = &u.ID
		}
		counters = append(counters, models.Counter{
			ID:       uint(i + 7),
			Name:     fmt.Sprintf("Loket %02d - Pendaftaran Sekolah", i + 7),
			RoomType: models.StepInputRoom,
			IsActive: true,
			StaffID:  staffID,
		})
	}

	for _, c := range counters {
		var existing models.Counter
		err := db.First(&existing, c.ID).Error
		if err != nil {
			// Belum ada, buat baru
			db.Create(&c)
			log.Printf("✅ Loket '%s' berhasil dibuat\n", c.Name)
		} else {
			// Update loket yang sudah ada agar sesuai mapping baru
			existing.Name = c.Name
			existing.RoomType = c.RoomType
			existing.StaffID = c.StaffID
			existing.IsActive = c.IsActive
			db.Save(&existing)
			log.Printf("✅ Loket '%s' berhasil diperbarui\n", c.Name)
		}
	}

	log.Println("✨ Seeding data selesai!")
}
