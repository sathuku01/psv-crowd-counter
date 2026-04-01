package handlers

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"psv-crowd-counter/frontend/internal/models"
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
	templateDir := "frontend/internal/templates"

	// Define template files
	templateFiles := map[string]string{
		"dashboard": filepath.Join(templateDir, "dashboard.html"),
		"vehicles":  filepath.Join(templateDir, "vehicles.html"),
		"analytics": filepath.Join(templateDir, "analytics.html"),
		"error":     filepath.Join(templateDir, "error.html"),
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

	h.renderTemplate(w, "dashboard", data)
}

// VehiclesHandler handles the vehicles page
func (h *Handler) VehiclesHandler(w http.ResponseWriter, r *http.Request) {
	data, err := h.apiService.GetDashboardData()
	if err != nil {
		log.Printf("Error fetching vehicles data: %v", err)
		h.renderError(w, "Failed to load vehicles data", err.Error())
		return
	}

	h.renderTemplate(w, "vehicles", data)
}

// AnalyticsHandler handles the analytics page
func (h *Handler) AnalyticsHandler(w http.ResponseWriter, r *http.Request) {
	// Parse filter parameters from query string
	filter := &models.FilterParams{
		VehicleID: r.URL.Query().Get("vehicle_id"),
		StartTime: r.URL.Query().Get("start_time"),
		EndTime:   r.URL.Query().Get("end_time"),
	}

	data, err := h.apiService.GetAnalyticsData(filter)
	if err != nil {
		log.Printf("Error fetching analytics data: %v", err)
		h.renderError(w, "Failed to load analytics data", err.Error())
		return
	}

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
	// Simple JSON encoding
	w.Write([]byte("["))
	for i, report := range reports {
		if i > 0 {
			w.Write([]byte(","))
		}
		w.Write([]byte(`{"bus_id":"` + report.BusID + `","front":` + string(rune(report.Front+'0')) + `,"rear":` + string(rune(report.Rear+'0')) + `,"speed_kph":` + string(rune(int(report.SpeedKPH)+'0')) + `}`))
	}
	w.Write([]byte("]"))
}

// APIHealthHandler handles API health check requests
func (h *Handler) APIHealthHandler(w http.ResponseWriter, r *http.Request) {
	health, err := h.apiService.GetHealth()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","backend":`))
	if health != nil {
		w.Write([]byte(`"connected"`))
	} else {
		w.Write([]byte(`"disconnected"`))
	}
	w.Write([]byte(`}`))
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
