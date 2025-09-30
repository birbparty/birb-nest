package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/birbparty/birb-nest/internal/api"
	"github.com/birbparty/birb-nest/internal/cache"
	"github.com/birbparty/birb-nest/internal/database"
	"github.com/birbparty/birb-nest/internal/instance"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	// Load API configuration
	cfg, err := api.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("🐦 Birb Nest API starting in %s mode (instance: %s)...", cfg.Mode, cfg.InstanceID)

	// Initialize Redis cache
	cacheConfig := &cache.Config{
		Host:     cfg.Redis.Host,
		Port:     cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}

	redisCache, err := cache.NewRedisCache(cacheConfig)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisCache.Close()
	log.Println("✅ Connected to Redis")

	// Initialize instance registry
	registry := instance.NewRegistry(redisCache)
	log.Println("✅ Initialized instance registry")

	// Initialize default instance
	ctx := context.Background()
	defaultInst, err := registry.GetOrCreate(ctx, cfg.DefaultInstanceID)
	if err != nil {
		log.Printf("Warning: Failed to initialize default instance: %v", err)
	} else {
		// Mark default instance as permanent
		if !defaultInst.IsPermanent {
			defaultInst.IsPermanent = true
			defaultInst.Metadata["type"] = "default"
			defaultInst.Metadata["created_by"] = "system"
			if err := registry.Update(ctx, defaultInst); err != nil {
				log.Printf("Warning: Failed to update default instance: %v", err)
			} else {
				log.Printf("✅ Initialized default instance: %s", cfg.DefaultInstanceID)
				api.RecordDefaultInstanceCreated()
			}
		} else {
			log.Printf("✅ Default instance already exists: %s", cfg.DefaultInstanceID)
		}
	}

	// Initialize database only for primary mode
	var db database.Interface
	if cfg.IsPrimary() && cfg.PostgreSQL.Enabled {
		dbConfig, err := database.NewConfigFromEnv()
		if err != nil {
			log.Fatalf("Failed to load database configuration: %v", err)
		}

		db, err = database.NewPostgreSQLClient(dbConfig, cfg.InstanceID)
		if err != nil {
			log.Fatalf("Failed to connect to PostgreSQL: %v", err)
		}
		defer db.Close()
		log.Println("✅ Connected to PostgreSQL")
	}

	// Create handlers with mode awareness
	handlers := api.NewHandlers(cfg, redisCache, db, registry)
	defer handlers.Shutdown()

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:               fmt.Sprintf("Birb Nest API - %s", cfg.Mode),
		ErrorHandler:          fiber.DefaultErrorHandler,
		ReadTimeout:           time.Duration(cfg.RequestTimeout) * time.Second,
		WriteTimeout:          time.Duration(cfg.RequestTimeout) * time.Second,
		IdleTimeout:           120 * time.Second,
		DisableStartupMessage: true,
	})

	// Setup middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format:     "[${time}] ${status} - ${latency} ${method} ${path} ${error}\n",
		TimeFormat: "2006-01-02 15:04:05",
		TimeZone:   "UTC",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Instance-ID, X-Write-Timestamp",
	}))

	// Setup routes
	api.SetupInstanceRoutes(app, handlers, cfg, registry)

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("🛑 Shutting down gracefully...")

		// Give the server time to shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Duration(cfg.ShutdownTimeout)*time.Second)
		defer shutdownCancel()

		// Shutdown handlers first (to stop async writer)
		handlers.Shutdown()

		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}
	}()

	// Log instance information
	if cfg.IsReplica() {
		log.Printf("📡 Replica instance configured to forward writes to: %s", cfg.PrimaryURL)
	}

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	log.Printf("🚀 Birb Nest API (%s) listening on %s", cfg.Mode, addr)

	if err := app.Listen(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
