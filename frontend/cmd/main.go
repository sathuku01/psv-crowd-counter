package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"text/template"

	"psv-crowd-counter/frontend/internal/handlers"
	"psv-crowd-counter/frontend/internal/services"
)

// Template functions
var templateFuncs = template.FuncMap{
	"add": func(a, b int) int {
		return a + b
	},
	"sub": func(a, b int) int {
		return a - b
	},
	"mul": func(a, b float64) float64 {
		return a * b
	},
}

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

	// Create router
	mux := http.NewServeMux()

	// Page routes
	mux.HandleFunc("/", handler.DashboardHandler)
	mux.HandleFunc("/analytics", handler.AnalyticsHandler)

	// API routes
	mux.HandleFunc("/api/reports", handler.APIReportsHandler)
	mux.HandleFunc("/api/health", handler.APIHealthHandler)

	// Static file server
	// Get the directory of the current source file
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Printf("Warning: Could not get source file path")
	} else {
		// Get the directory containing this file
		currentDir := filepath.Dir(filename)

		// Navigate to the static directory (go up one level to frontend, then to static)
		staticDir := filepath.Join(currentDir, "..", "static")

		if _, err := os.Stat(staticDir); os.IsNotExist(err) {
			log.Printf("Warning: Static directory '%s' not found", staticDir)
		} else {
			fs := http.FileServer(http.Dir(staticDir))
			mux.Handle("/static/", http.StripPrefix("/static/", fs))
		}
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
