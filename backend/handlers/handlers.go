package handlers

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"17an/service"
)

type Handler struct {
	UserService service.UserService
}

func New(userService service.UserService) *Handler {
	return &Handler{UserService: userService}
}

func (h *Handler) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

// ListUsers godoc
//
//	@Summary		List users (paginated)
//	@Tags			users
//	@Produce		json
//	@Param			page	query		int	false	"default 1"
//	@Param			limit	query		int	false	"default 20, max 100"
//	@Success		200	{object}	map[string]interface{}
//	@Router			/api/v1/users [get]
func (h *Handler) ListUsers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	users, err := h.UserService.ListUsers(page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"page": page, "limit": limit, "data": users})
}

// GetUser godoc
//
//	@Summary		Get one user by ID
//	@Tags			users
//	@Produce		json
//	@Param			id	path		int	true	"user_id"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	map[string]interface{}
//	@Router			/api/v1/users/{id} [get]
func (h *Handler) GetUser(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	user, err := h.UserService.GetUser(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"data": user})
}
