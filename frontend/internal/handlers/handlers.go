package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

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
	"div": func(a, b float64) float64 {
		if b == 0 {
			return 0
		}
		return a / b
	},
	"float64": func(i int) float64 {
		return float64(i)
	},
}

// Handler holds the dependencies for HTTP handlers
type Handler struct {
	apiService *services.APIService
	templates  map[string]*template.Template
}

// NewHandler creates a new handler instance
func NewHandler(apiService *services.APIService) *Handler {
	h := &Handler{
		apiService: apiService,
		templates:  make(map[string]*template.Template),
	}
	h.loadTemplates()
	return h
}

// loadTemplates loads all HTML templates
func (h *Handler) loadTemplates() {
	// Get the current working directory
	execDir, err := os.Getwd()
	if err != nil {
		log.Printf("Warning: Could not get working directory: %v", err)
		return
	}

	// Try multiple paths to find the templates directory
	templatePaths := []string{
		filepath.Join(execDir, "frontend", "internal", "templates"),
		filepath.Join(execDir, "..", "frontend", "internal", "templates"),
		filepath.Join(execDir, "..", "..", "frontend", "internal", "templates"),
	}

	var templateDir string
	for _, path := range templatePaths {
		if _, err := os.Stat(path); err == nil {
			templateDir = path
			break
		}
	}

	if templateDir == "" {
		log.Printf("Warning: Templates directory not found in any of these paths: %v", templatePaths)
		return
	}

	// Define template files
	templateFiles := map[string]string{
		"dashboard": filepath.Join(templateDir, "dashboard.html"),
		"analytics": filepath.Join(templateDir, "analytics.html"),
		"error":     filepath.Join(templateDir, "error.html"),
		"vehicles":  filepath.Join(templateDir, "vehicles.html"),
	}

	// Parse templates with layout and custom functions
	for name, file := range templateFiles {
		tmpl, err := template.New(name).Funcs(templateFuncs).ParseFiles(file, filepath.Join(templateDir, "layout.html"))
		if err != nil {
			log.Printf("Warning: Failed to load template %s: %v", name, err)
			continue
		}
		h.templates[name] = tmpl
	}
}

// DashboardHandler handles the dashboard page
func (h *Handler) DashboardHandler(w http.ResponseWriter, r *http.Request) {
	data, err := h.apiService.GetDashboardData()
	if err != nil {
		log.Printf("Error fetching dashboard data: %v", err)
		h.renderError(w, "Failed to load dashboard data", err.Error())
		return
	}

	// Add MediaPipe and Crowd Detection URLs to data
	data.MEDIAPIPE_URL = h.apiService.GetMediaPipeURL()
	data.CROWD_DETECT_URL = h.apiService.GetCrowdDetectURL()

	h.renderTemplate(w, "dashboard", data)
}

// AnalyticsHandler handles the analytics page
func (h *Handler) AnalyticsHandler(w http.ResponseWriter, r *http.Request) {
	data, err := h.apiService.GetAnalyticsData()
	if err != nil {
		log.Printf("Error fetching analytics data: %v", err)
		h.renderError(w, "Failed to load analytics data", err.Error())
		return
	}

	// Add MediaPipe and Crowd Detection URLs to data
	data.MEDIAPIPE_URL = h.apiService.GetMediaPipeURL()
	data.CROWD_DETECT_URL = h.apiService.GetCrowdDetectURL()

	h.renderTemplate(w, "analytics", data)
}

// APIReportsHandler handles API requests for reports (JSON)
func (h *Handler) APIReportsHandler(w http.ResponseWriter, r *http.Request) {
	reports, err := h.apiService.GetReports()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Use proper JSON encoding with json.Marshal
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(reports); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// APIHealthHandler handles API health check requests
func (h *Handler) APIHealthHandler(w http.ResponseWriter, r *http.Request) {
	health, err := h.apiService.GetHealth()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Use proper JSON encoding
	response := map[string]interface{}{
		"status":  "ok",
		"backend": "connected",
	}
	if health == nil {
		response["backend"] = "disconnected"
	}

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// VehiclesHandler handles the vehicles page
func (h *Handler) VehiclesHandler(w http.ResponseWriter, r *http.Request) {
	data, err := h.apiService.GetDashboardData()
	if err != nil {
		log.Printf("Error fetching vehicles data: %v", err)
		h.renderError(w, "Failed to load vehicles data", err.Error())
		return
	}

	// Add MediaPipe URL to data
	data.MEDIAPIPE_URL = h.apiService.GetMediaPipeURL()

	h.renderTemplate(w, "vehicles", data)
}

// APIAnalyticsHandler handles API requests for analytics data (JSON)
func (h *Handler) APIAnalyticsHandler(w http.ResponseWriter, r *http.Request) {
	analytics, err := h.apiService.GetAnalyticsData()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(analytics); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// renderTemplate renders a template with the given data
func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl, ok := h.templates[name]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("Error executing template %s: %v", name, err)
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}

// renderError renders an error page
func (h *Handler) renderError(w http.ResponseWriter, title, message string) {
	data := map[string]string{
		"Title":   title,
		"Message": message,
	}

	tmpl, ok := h.templates["error"]
	if !ok {
		http.Error(w, title+": "+message, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("Error executing error template: %v", err)
		http.Error(w, title+": "+message, http.StatusInternalServerError)
	}
}
