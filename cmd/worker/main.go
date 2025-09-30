package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/birbparty/birb-nest/internal/cache"
	"github.com/birbparty/birb-nest/internal/cleanup"
	"github.com/birbparty/birb-nest/internal/database"
	"github.com/birbparty/birb-nest/internal/instance"
	"github.com/birbparty/birb-nest/internal/operations"
	"github.com/birbparty/birb-nest/internal/queue"
	"github.com/birbparty/birb-nest/internal/storage"
	"github.com/birbparty/birb-nest/internal/worker"
)

func main() {
	log.Println("🐦 Birb Nest Worker starting...")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize configurations
	workerConfig, err := worker.NewConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to load worker config: %v", err)
	}

	dbConfig, err := database.NewConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to load database config: %v", err)
	}

	cacheConfig, err := cache.NewConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to load cache config: %v", err)
	}

	queueConfig, err := queue.NewConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to load queue config: %v", err)
	}

	// Initialize database
	db, err := database.NewDB(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("✅ Connected to PostgreSQL")

	// Initialize Redis cache
	redisCache, err := cache.NewRedisCache(cacheConfig)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisCache.Close()
	log.Println("✅ Connected to Redis")

	// Initialize NATS queue
	queueClient, err := queue.NewClient(queueConfig)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer queueClient.Close()
	log.Println("✅ Connected to NATS JetStream")

	// Initialize metrics
	metrics := worker.NewMetrics()

	// Create processor
	processor := worker.NewProcessor(workerConfig, db, redisCache, queueClient, metrics)

	// Initialize instance registry
	registry := instance.NewRegistry(redisCache)

	// Initialize operations handler
	instanceOps := operations.NewInstanceOperations(redisCache, db, registry)

	// Initialize storage client for archival (optional)
	var spacesClient *storage.SpacesClient
	spacesConfig := cleanup.LoadSpacesConfig()
	if spacesConfig.AccessKey != "" && spacesConfig.SecretKey != "" {
		var err error
		spacesClient, err = storage.NewSpacesClient(spacesConfig)
		if err != nil {
			log.Printf("Warning: Failed to initialize Spaces client: %v. Archival will be disabled.", err)
		} else {
			log.Println("✅ Connected to Digital Ocean Spaces for archival")
		}
	} else {
		log.Println("⚠️ Spaces credentials not configured. Archival will be disabled.")
	}

	// Initialize cleanup service
	cleanupConfig := cleanup.LoadCleanupConfig()
	if spacesClient == nil && cleanupConfig.ArchiveBeforeDelete {
		log.Println("⚠️ Archive before delete is enabled but Spaces is not configured. Disabling archival.")
		cleanupConfig.ArchiveBeforeDelete = false
	}

	cleanupService := cleanup.NewCleanupService(
		registry,
		instanceOps,
		spacesClient,
		queueClient.GetNC(), // Get the underlying NATS connection
		cleanupConfig,
	)

	// Start health check server
	go startHealthServer(workerConfig.HealthCheckPort, metrics)

	// Perform startup rehydration
	if err := processor.PerformStartupRehydration(ctx); err != nil {
		log.Printf("Warning: Startup rehydration failed: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start processing in background
	processorDone := make(chan error, 1)
	go func() {
		processorDone <- processor.Start(ctx)
	}()

	// Start cleanup service in background
	go func() {
		log.Println("🧹 Starting cleanup service...")
		cleanupService.Start(ctx)
	}()

	// Wait for shutdown signal or processor error
	select {
	case sig := <-sigChan:
		log.Printf("🛑 Received signal: %v. Shutting down gracefully...", sig)
		cancel()
		processor.Stop()

		// Wait for processor to finish with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		select {
		case <-processorDone:
			log.Println("✅ Worker shutdown complete")
		case <-shutdownCtx.Done():
			log.Println("⚠️ Worker shutdown timeout")
		}

	case err := <-processorDone:
		if err != nil {
			log.Fatalf("Processor error: %v", err)
		}
	}
}

func startHealthServer(port int, metrics *worker.Metrics) {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if metrics.IsHealthy() {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"healthy","service":"birb-nest-worker"}`)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"unhealthy","service":"birb-nest-worker"}`)
		}
	})

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		stats := metrics.GetStats()

		// Convert to JSON
		response := fmt.Sprintf(`{
			"uptime_seconds": %v,
			"messages_processed": %v,
			"messages_succeeded": %v,
			"messages_failed": %v,
			"persistence_count": %v,
			"rehydration_count": %v,
			"avg_batch_size": %v,
			"avg_processing_time_ms": %v
		}`,
			stats["uptime_seconds"],
			stats["messages_processed"],
			stats["messages_succeeded"],
			stats["messages_failed"],
			stats["persistence_count"],
			stats["rehydration_count"],
			stats["avg_batch_size"],
			stats["avg_processing_time_ms"],
		)

		fmt.Fprint(w, response)
	})

	log.Printf("🏥 Health check server listening on port %d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Printf("Health server error: %v", err)
	}
}
