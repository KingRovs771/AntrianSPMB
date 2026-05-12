package handler

import (
	"AntrianSPMB/internal/models"
	"AntrianSPMB/internal/services"
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type AuthHandler struct {
	authService service.AuthService
}

func NewAuthHandler(as service.AuthService) *AuthHandler {
	return &AuthHandler{authService: as}
}

// HandleLogin memproses form login dari HTMX
func (h *AuthHandler) HandleLogin(c *fiber.Ctx) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	user, err := h.authService.Login(username, password)
	if err != nil {
		// Jika gagal, kembalikan pesan error untuk HTMX
		return c.SendString(`<span class="text-red-500 font-bold text-sm">Username atau password salah!</span>`)
	}

	// Buat JWT Token
	token, err := h.authService.GenerateToken(user)
	if err != nil {
		return c.SendString(`<span class="text-red-500 font-bold text-sm">Gagal membuat sesi login!</span>`)
	}

	// Login sukses: Simpan info di Cookie dalam bentuk JWT
	c.Cookie(&fiber.Cookie{
		Name:     "jwt_token",
		Value:    token,
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: true,
		Path:     "/",
		// Secure: true, // Aktifkan jika menggunakan HTTPS
	})

	// Simpan username di cookie tambahan (opsional, untuk UI saja)
	c.Cookie(&fiber.Cookie{
		Name:    "session_user",
		Value:   user.Username,
		Expires: time.Now().Add(24 * time.Hour),
		Path:    "/",
	})

	// Redirect HTMX ke dashboard sesuai Role
	if user.Role == models.RoleAdmin {
		c.Set("HX-Redirect", "/admin/dashboard")
	} else {
		c.Set("HX-Redirect", "/dashboard/loket")
	}
	return c.SendStatus(fiber.StatusOK)
}

// HandleLogout menghapus cookie jwt
func (h *AuthHandler) HandleLogout(c *fiber.Ctx) error {
	// Hapus cookie dengan path yang sama saat dibuat (root)
	c.Cookie(&fiber.Cookie{
		Name:     "jwt_token",
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		Path:     "/",
		HTTPOnly: true,
	})
	c.Cookie(&fiber.Cookie{
		Name:    "session_user",
		Value:   "",
		Expires: time.Now().Add(-time.Hour),
		Path:    "/",
	})
	
	// Jika dipanggil via HTMX, gunakan header khusus untuk redirect halaman penuh
	if c.Get("HX-Request") != "" {
		c.Set("HX-Redirect", "/login")
		return c.SendStatus(fiber.StatusOK)
	}
	
	return c.Redirect("/login")
}

// AuthMiddleware melindungi rute yang membutuhkan login menggunakan JWT
func AuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenString := c.Cookies("jwt_token")
		if tokenString == "" {
			return redirectToLogin(c)
		}

		// Validasi JWT
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			secret = "default_secret_key"
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("metode enkripsi tidak sesuai: %v", token.Header["alg"])
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.Cookie(&fiber.Cookie{
				Name:     "jwt_token",
				Value:    "",
				Expires:  time.Now().Add(-time.Hour),
				Path:     "/",
				HTTPOnly: true,
			})
			return redirectToLogin(c)
		}

		// Ambil data dari claims (opsional, jika ingin menyimpan data user ke Locals)
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			c.Locals("user_id", claims["id"])
			c.Locals("username", claims["username"])
			c.Locals("role", claims["role"])
		}

		return c.Next()
	}
}

// RoleMiddleware membatasi akses berdasarkan role tertentu
func RoleMiddleware(requiredRole string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role := c.Locals("role")
		if role != requiredRole {
			return c.Status(fiber.StatusForbidden).SendString("Anda tidak memiliki izin untuk mengakses halaman ini.")
		}
		return c.Next()
	}
}

func redirectToLogin(c *fiber.Ctx) error {
	if c.Get("HX-Request") != "" {
		c.Set("HX-Redirect", "/login")
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	return c.Redirect("/login")
}
