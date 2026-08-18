// Package main implements the 17an Customer Intelligence API.
//
//	@title			17an Customer Intelligence API
//	@version		1.0
//	@description	Search, data quality, duplicate detection, and cross-table profile
//	@description	lookups over a 15M-row customer dataset (CHALLENGE.md Rounds 1-5).
//	@BasePath		/
package main

import (
	"context"
	"log"

	swagger "github.com/swaggo/fiber-swagger"
	"go.opentelemetry.io/otel/trace"

	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	"17an/config"
	"17an/database"
	_ "17an/docs"
	"17an/handlers"
	"17an/repository"
	"17an/routes"
	"17an/service"
	"17an/tracing"
)

func main() {
	cfg := config.Load()

	shutdownTracing, err := tracing.Setup(context.Background())
	if err != nil {
		log.Fatalf("failed to set up tracing: %v", err)
	}
	defer shutdownTracing(context.Background())

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	app := fiber.New()
	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(otelfiber.Middleware())
	app.Use(func(c *fiber.Ctx) error {
		if span := trace.SpanContextFromContext(c.UserContext()); span.HasTraceID() {
			c.Locals("traceID", span.TraceID().String())
		}
		return c.Next()
	})
	app.Use(logger.New(logger.Config{
		// Correlates every access-log line with its request ID and OTel trace ID,
		// so a slow/error line here can be pasted straight into the Jaeger search box.
		Format: "${time} | ${status} | ${latency} | ${ip} | ${method} | ${path} | reqid=${locals:requestid} trace=${locals:traceID}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	app.Get("/swagger/*", swagger.WrapHandler)

	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	h := handlers.New(userService)

	searchRepo := repository.NewSearchRepository(db)
	analyticsRepo := repository.NewAnalyticsRepository(db)
	apiHandler := handlers.NewAPIHandler(
		service.NewHealthService(db),
		service.NewSearchService(searchRepo),
		service.NewQualityService(analyticsRepo),
		service.NewDuplicateService(analyticsRepo),
		service.NewProfileService(analyticsRepo),
	)

	routes.Setup(app, h, apiHandler)

	log.Fatal(app.Listen(":" + cfg.Port))
}
