package handlers

import (
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"17an/service"
)

type APIHandler struct {
	HealthSvc    service.HealthService
	SearchSvc    service.SearchService
	QualitySvc   service.QualityService
	DuplicateSvc service.DuplicateService
	ProfileSvc   service.ProfileService
}

func NewAPIHandler(health service.HealthService, search service.SearchService, quality service.QualityService, dup service.DuplicateService, profile service.ProfileService) *APIHandler {
	return &APIHandler{HealthSvc: health, SearchSvc: search, QualitySvc: quality, DuplicateSvc: dup, ProfileSvc: profile}
}

func (h *APIHandler) HealthCheck(c *fiber.Ctx) error {
	result, err := h.HealthSvc.Check(c.Context())
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"ok": false, "status": "error", "database": "disconnected", "timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
	return c.JSON(result)
}

func (h *APIHandler) Search(c *fiber.Ctx) error {
	start := time.Now()

	q := c.Query("q")
	searchType := c.Query("type")
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	if limit < 1 || limit > 100 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	if q == "" || searchType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "q and type are required"})
	}

	results, total, err := h.SearchSvc.Search(searchType, q, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	out := make([]fiber.Map, 0, len(results))
	for _, r := range results {
		out = append(out, fiber.Map{
			"user_id":    r.UserID,
			"full_name":  r.FullName,
			"user_email": r.UserEmail,
			"msisdn":     r.Msisdn,
			"status":     r.Status,
			"created_at": r.CreatedAt,
		})
	}

	return c.JSON(fiber.Map{
		"query":   q,
		"type":    searchType,
		"limit":   limit,
		"offset":  offset,
		"results": out,
		"total":   total,
		"took_ms": time.Since(start).Milliseconds(),
	})
}

func (h *APIHandler) Quality(c *fiber.Ctx) error {
	result, err := h.QualitySvc.Compute()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func (h *APIHandler) Metrics(c *fiber.Ctx) error {
	result, err := h.QualitySvc.Metrics()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func (h *APIHandler) DuplicatesPost(c *fiber.Ctx) error {
	var body struct {
		Limit int `json:"limit"`
	}
	_ = c.BodyParser(&body)
	if body.Limit < 1 || body.Limit > 1000 {
		body.Limit = 100
	}
	result, err := h.DuplicateSvc.ExactPairs(body.Limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func (h *APIHandler) DuplicatesFind(c *fiber.Ctx) error {
	method := c.Query("method", "ip_address")
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if limit < 1 || limit > 500 {
		limit = 50
	}
	if method != "ip_address" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported method"})
	}
	result, err := h.DuplicateSvc.FindByIP(limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func (h *APIHandler) DuplicatesByUser(c *fiber.Ctx) error {
	userID, err := strconv.ParseUint(c.Params("user_id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user_id"})
	}
	result, err := h.DuplicateSvc.ForUser(userID, 50)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func (h *APIHandler) UserProfile(c *fiber.Ctx) error {
	userID, err := strconv.ParseUint(c.Params("user_id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user_id"})
	}
	p, err := h.ProfileSvc.Get(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"user_id":                  p.UserID,
		"full_name":                p.FullName,
		"user_email":               p.UserEmail,
		"msisdn":                   p.Msisdn,
		"order_count":              p.OrderCount,
		"total_transaction_amount": p.TotalTransactionAmount,
		"activity_count":           p.ActivityCount,
		"last_activity":            p.LastActivity,
	})
}
