package utils

import "github.com/gofiber/fiber/v2"

// BaseResponse adalah format standar untuk setiap balasan JSON API dari server kita.
// Ini memudahkan sisi client/frontend untuk memproses (parsing) balasan dari backend.
type BaseResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`

	// omitempty berarti key ini tidak akan muncul di JSON jika nilainya kosong (nil)
	Data   interface{} `json:"data,omitempty"`
	Errors interface{} `json:"errors,omitempty"`
}

// SuccessResponse adalah fungsi pembantu (helper) untuk merender JSON jika operasi berhasil.
// Contoh penggunaan di handler:
// return utils.SuccessResponse(c, 200, "Tiket berhasil dilewati", nil)
func SuccessResponse(c *fiber.Ctx, statusCode int, message string, data interface{}) error {
	return c.Status(statusCode).JSON(BaseResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// ErrorResponse adalah fungsi pembantu untuk merender JSON jika terjadi kesalahan.
// Contoh penggunaan di handler:
// return utils.ErrorResponse(c, 404, "Loket tidak ditemukan", err.Error())
func ErrorResponse(c *fiber.Ctx, statusCode int, message string, errors interface{}) error {
	return c.Status(statusCode).JSON(BaseResponse{
		Success: false,
		Message: message,
		Errors:  errors,
	})
}

// HTMXRedirect adalah fungsi pembantu khusus untuk HTMX.
// Karena HTMX tidak mengikuti redirect standar HTTP (301/302), kita harus mengirimkan
// header HX-Redirect agar browser berpindah halaman.
func HTMXRedirect(c *fiber.Ctx, url string) error {
	c.Set("HX-Redirect", url)
	return c.SendStatus(fiber.StatusOK)
}

// HTMXTrigger mengirimkan header trigger event khusus untuk HTMX.
// Sangat berguna untuk menyuruh komponen lain di UI (seperti tabel) merefresh dirinya sendiri.
func HTMXTrigger(c *fiber.Ctx, eventName string) {
	c.Set("HX-Trigger", eventName)
}
