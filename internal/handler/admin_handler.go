package handler

import (
	"AntrianSPMB/internal/models"
	"AntrianSPMB/internal/services"
	"AntrianSPMB/pkg/sse"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type AdminHandler struct {
	userService  service.UserService
	queueService service.QueueService
	sseManager   *sse.Manager
}

func NewAdminHandler(us service.UserService, qs service.QueueService, sm *sse.Manager) *AdminHandler {
	return &AdminHandler{
		userService:  us,
		queueService: qs,
		sseManager:   sm,
	}
}

// Dashboard View
func (h *AdminHandler) Dashboard(c *fiber.Ctx) error {
	todayCount, _ := h.queueService.CountToday()
	totalCount, _ := h.queueService.CountTotalAll()

	return c.Render("pages/admin_dashboard", fiber.Map{
		"Title":      "Admin Dashboard",
		"TodayCount": todayCount,
		"TotalCount": totalCount,
		"Username":   c.Cookies("session_user"),
	})
}

// User List View (Full Page)
func (h *AdminHandler) UserList(c *fiber.Ctx) error {
	users, err := h.userService.GetAllUsers()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.Render("pages/admin_users", fiber.Map{
		"Title":    "Manajemen User",
		"Users":    users,
		"Username": c.Cookies("session_user"),
	})
}

// userRowsPartial renders just the <tr> rows for HTMX partial swap
func (h *AdminHandler) userRowsPartial(c *fiber.Ctx) error {
	users, err := h.userService.GetAllUsers()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	// Render partial with empty layout so only the rows are returned
	return c.Render("partials/admin_user_rows", users, "")
}

// Create User
func (h *AdminHandler) CreateUser(c *fiber.Ctx) error {
	user := new(models.User)
	if err := c.BodyParser(user); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := h.userService.CreateUser(user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return h.userRowsPartial(c)
}

// Update User
func (h *AdminHandler) UpdateUser(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	user := new(models.User)
	if err := c.BodyParser(user); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	user.ID = uint(id)
	if err := h.userService.UpdateUser(user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return h.userRowsPartial(c)
}

// Delete User
func (h *AdminHandler) DeleteUser(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	if err := h.userService.DeleteUser(uint(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return h.userRowsPartial(c)
}

// Reset Queue
func (h *AdminHandler) ResetQueue(c *fiber.Ctx) error {
	if err := h.queueService.ResetQueues(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Broadcast SSE ke Monitor TV dan HP Murid
	if h.sseManager != nil {
		h.sseManager.Broadcast("monitor_active", "trigger", nil)
		h.sseManager.Broadcast("all", "status_updated", nil)
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *AdminHandler) SetupAdminRoutes(router fiber.Router) {
	admin := router.Group("/admin", AuthMiddleware(), RoleMiddleware("ADMIN"))

	admin.Get("/dashboard", h.Dashboard)
	admin.Get("/users", h.UserList)
	admin.Post("/users", h.CreateUser)
	admin.Put("/users/:id", h.UpdateUser)
	admin.Delete("/users/:id", h.DeleteUser)
	admin.Post("/reset", h.ResetQueue)
}

