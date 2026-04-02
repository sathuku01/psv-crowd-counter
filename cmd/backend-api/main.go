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

	"psv-crowd-counter/internal/api/config"
	"psv-crowd-counter/internal/api/router"
	mockcam "psv-crowd-counter/internal/camera/mock"
	mockdet "psv-crowd-counter/internal/detector/mock"
	"psv-crowd-counter/internal/service"
	"psv-crowd-counter/internal/storage/jsonstore"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Get data directory from environment or use default
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Initialize storage
	reportsPath := fmt.Sprintf("%s/reports.json", dataDir)
	store := jsonstore.New(reportsPath)

	// Initialize camera (mock for now)
	camera := mockcam.NewMockCamera(1 * time.Second)

	// Initialize detector (mock for now)
	detector := mockdet.NewMockDetector()

	// Initialize processor
	busID := os.Getenv("BUS_ID")
	if busID == "" {
		busID = "BUS-001"
	}
	reportInterval := 1 * time.Minute
	processor := service.NewProcessor(camera, detector, store, busID, reportInterval)

	// Start processor in background
	processor.Start()
	log.Printf("Processor started for bus %s", busID)

	// Initialize router with all routes and middleware
	r := router.NewRouter(cfg, store, processor)
	handler := r.SetupRoutes()

	// Create HTTP server with timeouts
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Server.Port),
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Backend API server starting on http://localhost%s", server.Addr)
		log.Printf("Health endpoint: http://localhost%s/api/v1/health", server.Addr)
		log.Printf("Reports endpoint: http://localhost%s/api/v1/reports", server.Addr)
		log.Printf("Bus status endpoint: http://localhost%s/api/v1/buses/status", server.Addr)
		log.Printf("Analytics endpoint: http://localhost%s/api/v1/analytics", server.Addr)
		log.Printf("API Key: %s", cfg.Security.APIKey)

		var err error
		if cfg.Security.EnableHTTPS {
			log.Printf("HTTPS enabled")
			err = server.ListenAndServeTLS(cfg.Security.TLSCertFile, cfg.Security.TLSKeyFile)
		} else {
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Create a deadline for server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}
