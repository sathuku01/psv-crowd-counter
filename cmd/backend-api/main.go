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
	"psv-crowd-counter/internal/camera"
	"psv-crowd-counter/internal/detector"
	"psv-crowd-counter/internal/service"
	"psv-crowd-counter/internal/storage/jsonstore"

	mock "psv-crowd-counter/internal/camera/mock"
	mockdet "psv-crowd-counter/internal/detector/mock"
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

	// Create detections channel for websocket
	detections := make(chan detector.Result, 10)

	// Check if we should use mock implementations (for cloud deployment)
	useMock := os.Getenv("USE_MOCK_IMPLEMENTATIONS") == "true"

	var cameraImpl camera.Camera
	var detectorImpl detector.Detector

	if useMock {
		log.Println("Using mock implementations for cloud deployment")
		cameraImpl = mock.NewMockCamera(1 * time.Second)
		detectorImpl = mockdet.NewMockDetector()
	} else {
		// Initialize camera with real webcam processing
		cameraImpl = camera.NewRealCamera(0, 1*time.Second)

		// Initialize detector with real crowd detection
		detectorImpl = detector.NewRealDetector()
	}

	// Initialize processor
	busID := os.Getenv("BUS_ID")
	if busID == "" {
		busID = "BUS-001"
	}
	reportInterval := 10 * time.Second
	processor := service.NewProcessor(cameraImpl, detectorImpl, store, busID, reportInterval, detections)

	// Start processor in background
	processor.Start()
	log.Printf("Processor started for bus %s", busID)

	// Initialize router with all routes and middleware
	r := router.NewRouter(cfg, store, processor, detections)
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
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}
