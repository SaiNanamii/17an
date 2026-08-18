package routes

import (
	"github.com/gofiber/fiber/v2"

	"17an/handlers"
)

func Setup(app *fiber.App, h *handlers.Handler, a *handlers.APIHandler) {
	app.Get("/health", a.HealthCheck)

	api := app.Group("/api/v1")
	api.Get("/users", h.ListUsers)
	api.Get("/users/:id", h.GetUser)

	// CHALLENGE.md required endpoints
	app.Get("/api/health", a.HealthCheck)
	app.Get("/api/search", a.Search)
	app.Get("/api/quality", a.Quality)
	app.Get("/api/metrics", a.Metrics)
	app.Post("/api/duplicates", a.DuplicatesPost)
	app.Get("/api/duplicates/find", a.DuplicatesFind)
	app.Get("/api/duplicates/:user_id", a.DuplicatesByUser)
	app.Get("/api/user-profile/:user_id", a.UserProfile)
}
