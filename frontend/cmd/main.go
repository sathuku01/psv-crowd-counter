package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"psv-crowd-counter/frontend/internal/handlers"
	"psv-crowd-counter/frontend/internal/services"
)

func main() {
	// Get port from environment or use default
	port := os.Getenv("FRONTEND_PORT")
	if port == "" {
		port = "3000"
	}

	// Initialize services
	apiService := services.NewAPIService()

	// Initialize handlers
	handler := handlers.NewHandler(apiService)
	wsHandler := handlers.NewWebSocketHandler(apiService)

	// Create router
	mux := http.NewServeMux()

	// Page routes
	mux.HandleFunc("/", handler.DashboardHandler)
	mux.HandleFunc("/analytics", handler.AnalyticsHandler)
	mux.HandleFunc("/vehicles", handler.VehiclesHandler)

	// API routes
	mux.HandleFunc("/api/reports", handler.APIReportsHandler)
	mux.HandleFunc("/api/health", handler.APIHealthHandler)
	mux.HandleFunc("/api/v1/analytics", handler.APIAnalyticsHandler)

	// WebSocket route for real-time video processing
	mux.HandleFunc("/ws/detect", wsHandler.HandleWebSocket)

	// Static file server
	// Get the directory of the current executable or use working directory
	execDir, err := os.Getwd()
	if err != nil {
		log.Printf("Warning: Could not get working directory: %v", err)
	}

	// Try multiple paths to find the static directory
	staticPaths := []string{
		filepath.Join(execDir, "frontend", "static"),
		filepath.Join(execDir, "..", "static"),
		filepath.Join(execDir, "..", "..", "frontend", "static"),
	}

	var staticDir string
	for _, path := range staticPaths {
		if _, err := os.Stat(path); err == nil {
			staticDir = path
			break
		}
	}

	if staticDir == "" {
		log.Printf("Warning: Static directory not found in any of these paths: %v", staticPaths)
	} else {
		fs := http.FileServer(http.Dir(staticDir))
		mux.Handle("/static/", http.StripPrefix("/static/", fs))
	}

	// Start server
	addr := fmt.Sprintf(":%s", port)
	log.Printf("Frontend server starting on http://localhost%s", addr)
	log.Printf("Dashboard: http://localhost%s/", addr)
	log.Printf("API Health: http://localhost%s/api/health", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
