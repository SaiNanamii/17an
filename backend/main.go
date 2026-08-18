package main

import (
	"context"
	"log"

	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"17an/config"
	"17an/database"
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
	app.Use(logger.New())
	app.Use(cors.New())
	app.Use(otelfiber.Middleware())

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
