package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"psv-crowd-counter/internal/api/models"
	coremodels "psv-crowd-counter/internal/core/models"
	"psv-crowd-counter/internal/service"
	"psv-crowd-counter/internal/storage"
)

// Handler holds dependencies for API handlers
type Handler struct {
	store     storage.Store
	processor *service.Processor
	startTime time.Time
}

// NewHandler creates a new Handler instance
func NewHandler(store storage.Store, processor *service.Processor) *Handler {
	return &Handler{
		store:     store,
		processor: processor,
		startTime: time.Now(),
	}
}

// Health handles health check requests
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", "")
		return
	}

	response := models.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Version:   "1.0.0",
		Uptime:    time.Since(h.startTime).String(),
	}

	writeSuccess(w, http.StatusOK, response)
}

// GetReports handles retrieving all reports
func (h *Handler) GetReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", "")
		return
	}

	// Parse query parameters for filtering
	filter := parseReportFilter(r)

	reports, err := h.store.List()
	if err != nil {
		log.Printf("Error retrieving reports: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve reports", "")
		return
	}

	// Apply filters
	filteredReports := applyFilters(reports, filter)

	// Apply pagination
	paginatedReports, meta := paginate(filteredReports, filter.Page, filter.PerPage)

	// Convert to API response format
	apiReports := convertToAPIReports(paginatedReports)

	writeSuccessWithMeta(w, http.StatusOK, apiReports, meta)
}

// GetReportByID handles retrieving a specific report by ID
func (h *Handler) GetReportByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", "")
		return
	}

	// Extract ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/reports/")
	if path == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Report ID is required", "")
		return
	}

	reportID := path
	reports, err := h.store.List()
	if err != nil {
		log.Printf("Error retrieving reports: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve reports", "")
		return
	}

	for _, report := range reports {
		if generateReportID(report) == reportID {
			apiReport := convertToAPIReport(report)
			writeSuccess(w, http.StatusOK, apiReport)
			return
		}
	}

	writeError(w, http.StatusNotFound, "NOT_FOUND", "Report not found", fmt.Sprintf("No report found with ID: %s", reportID))
}

// GetBusStatus handles retrieving current bus status
func (h *Handler) GetBusStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", "")
		return
	}

	reports, err := h.store.List()
	if err != nil {
		log.Printf("Error retrieving reports: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve reports", "")
		return
	}

	// Group reports by bus ID and get latest status
	busStatuses := make(map[string]*models.BusStatus)
	for _, report := range reports {
		status, exists := busStatuses[report.BusID]
		if !exists || report.Timestamp.After(status.LastUpdated) {
			totalPassengers := report.Front + report.Rear
			busStatuses[report.BusID] = &models.BusStatus{
				BusID:         report.BusID,
				Passengers:    totalPassengers,
				LastUpdated:   report.Timestamp,
				IsActive:      time.Since(report.Timestamp) < 5*time.Minute,
				OccupancyRate: float64(totalPassengers) / 50.0, // Assuming max capacity of 50
			}
		}
	}

	// Convert map to slice
	statuses := make([]models.BusStatus, 0, len(busStatuses))
	for _, status := range busStatuses {
		statuses = append(statuses, *status)
	}

	writeSuccess(w, http.StatusOK, statuses)
}

// GetAnalytics handles retrieving analytics data
func (h *Handler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", "")
		return
	}

	reports, err := h.store.List()
	if err != nil {
		log.Printf("Error retrieving reports: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve reports", "")
		return
	}

	analytics := calculateAnalytics(reports)
	writeSuccess(w, http.StatusOK, analytics)
}

// CreateReport handles creating a new report
func (h *Handler) CreateReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", "")
		return
	}

	var req models.ReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON payload", err.Error())
		return
	}

	// Validate request
	if errors := validateReportRequest(req); len(errors) > 0 {
		writeValidationError(w, errors)
		return
	}

	// Create report using core model
	report := coremodels.Report{
		Timestamp: time.Now().UTC(),
		BusID:     req.BusID,
		Front:     req.Front,
		Rear:      req.Rear,
	}

	// Store report
	if err := h.store.Save(report); err != nil {
		log.Printf("Error saving report: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to save report", "")
		return
	}

	apiReport := convertToAPIReport(report)
	writeSuccess(w, http.StatusCreated, apiReport)
}

// GetProcessorStatus handles retrieving processor status
func (h *Handler) GetProcessorStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", "")
		return
	}

	status := map[string]interface{}{
		"status":    h.processor.Status(),
		"timestamp": time.Now().UTC(),
	}

	writeSuccess(w, http.StatusOK, status)
}

// Helper functions

func writeSuccess(w http.ResponseWriter, statusCode int, data interface{}) {
	response := models.APIResponse{
		Success: true,
		Data:    data,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func writeSuccessWithMeta(w http.ResponseWriter, statusCode int, data interface{}, meta *models.Meta) {
	response := models.APIResponse{
		Success: true,
		Data:    data,
		Meta:    meta,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func writeError(w http.ResponseWriter, statusCode int, code, message, details string) {
	response := models.APIResponse{
		Success: false,
		Error: &models.APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func writeValidationError(w http.ResponseWriter, errors []models.ValidationError) {
	details := make([]string, len(errors))
	for i, err := range errors {
		details[i] = fmt.Sprintf("%s: %s", err.Field, err.Message)
	}
	writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", strings.Join(details, "; "))
}

func parseReportFilter(r *http.Request) models.ReportFilter {
	query := r.URL.Query()
	filter := models.ReportFilter{
		BusID:   query.Get("bus_id"),
		Page:    1,
		PerPage: 20,
	}

	if page, err := strconv.Atoi(query.Get("page")); err == nil && page > 0 {
		filter.Page = page
	}

	if perPage, err := strconv.Atoi(query.Get("per_page")); err == nil && perPage > 0 && perPage <= 100 {
		filter.PerPage = perPage
	}

	if startTime, err := time.Parse(time.RFC3339, query.Get("start_time")); err == nil {
		filter.StartTime = startTime
	}

	if endTime, err := time.Parse(time.RFC3339, query.Get("end_time")); err == nil {
		filter.EndTime = endTime
	}

	return filter
}

func applyFilters(reports []coremodels.Report, filter models.ReportFilter) []coremodels.Report {
	filtered := make([]coremodels.Report, 0)
	for _, report := range reports {
		if filter.BusID != "" && report.BusID != filter.BusID {
			continue
		}
		if !filter.StartTime.IsZero() && report.Timestamp.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && report.Timestamp.After(filter.EndTime) {
			continue
		}

		filtered = append(filtered, report)
	}
	return filtered
}

func paginate(reports []coremodels.Report, page, perPage int) ([]coremodels.Report, *models.Meta) {
	total := len(reports)
	totalPages := (total + perPage - 1) / perPage

	start := (page - 1) * perPage
	end := start + perPage

	if start >= total {
		return []coremodels.Report{}, &models.Meta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		}
	}

	if end > total {
		end = total
	}

	return reports[start:end], &models.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	}
}

func calculateAnalytics(reports []coremodels.Report) models.Analytics {
	if len(reports) == 0 {
		return models.Analytics{
			TotalReports:      0,
			AveragePassengers: 0,

			PeakHour:           0,
			BusStats:           []models.BusStat{},
			HourlyDistribution: make(map[int]int),
			// Initialize drowsiness fields with default values
			EyeClosureAlerts:  0,
			YawningAlerts:     0,
			HeadPoseAlerts:    0,
			CriticalAlerts:    0,
			AverageEAR:        0.0,
			AverageMAR:        0.0,
			DetectionAccuracy: 0.0,
			ActiveSessions:    0,
		}
	}

	var totalPassengers float64
	busStats := make(map[string]*models.BusStat)
	hourlyDist := make(map[int]int)

	for _, report := range reports {
		passengers := report.Front + report.Rear
		totalPassengers += float64(passengers)

		// Bus stats
		stat, exists := busStats[report.BusID]
		if !exists {
			stat = &models.BusStat{BusID: report.BusID}
			busStats[report.BusID] = stat
		}
		stat.TotalReports++
		stat.AveragePassengers = (stat.AveragePassengers*float64(stat.TotalReports-1) + float64(passengers)) / float64(stat.TotalReports)
		if passengers > stat.MaxPassengers {
			stat.MaxPassengers = passengers
		}

		// Hourly distribution
		hour := report.Timestamp.Hour()
		hourlyDist[hour]++
	}

	// Find peak hour
	peakHour := 0
	maxCount := 0
	for hour, count := range hourlyDist {
		if count > maxCount {
			maxCount = count
			peakHour = hour
		}
	}

	// Convert bus stats map to slice
	busStatsSlice := make([]models.BusStat, 0, len(busStats))
	for _, stat := range busStats {
		busStatsSlice = append(busStatsSlice, *stat)
	}

	// Calculate drowsiness analytics (placeholder values for now)
	// In a real implementation, these would come from the MediaPipe server
	eyeClosureAlerts := len(reports) / 10 // Example: 10% of reports have eye closure alerts
	yawningAlerts := len(reports) / 15    // Example: 15% of reports have yawning alerts
	headPoseAlerts := len(reports) / 20   // Example: 20% of reports have head pose alerts
	criticalAlerts := len(reports) / 50   // Example: 50% of reports have critical alerts

	return models.Analytics{
		TotalReports:      len(reports),
		AveragePassengers: totalPassengers / float64(len(reports)),

		PeakHour:           peakHour,
		BusStats:           busStatsSlice,
		HourlyDistribution: hourlyDist,
		// Drowsiness fields
		EyeClosureAlerts:  eyeClosureAlerts,
		YawningAlerts:     yawningAlerts,
		HeadPoseAlerts:    headPoseAlerts,
		CriticalAlerts:    criticalAlerts,
		AverageEAR:        0.25,             // Default EAR value
		AverageMAR:        0.5,              // Default MAR value
		DetectionAccuracy: 95.5,             // Default accuracy percentage
		ActiveSessions:    len(reports) / 5, // Example: 5 reports per session
	}
}

func validateReportRequest(req models.ReportRequest) []models.ValidationError {
	errors := []models.ValidationError{}

	if req.BusID == "" {
		errors = append(errors, models.ValidationError{Field: "bus_id", Message: "Bus ID is required"})
	}

	if req.Front < 0 {
		errors = append(errors, models.ValidationError{Field: "front", Message: "Front count must be non-negative"})
	}

	if req.Rear < 0 {
		errors = append(errors, models.ValidationError{Field: "rear", Message: "Rear count must be non-negative"})
	}

	return errors
}

func generateReportID(report coremodels.Report) string {
	return fmt.Sprintf("%s-%d", report.BusID, report.Timestamp.UnixNano())
}

func convertToAPIReport(report coremodels.Report) models.APIReport {
	return models.APIReport{
		ID:         generateReportID(report),
		BusID:      report.BusID,
		Front:      report.Front,
		Rear:       report.Rear,
		Passengers: report.Front + report.Rear,

		Timestamp: report.Timestamp,
	}
}

func convertToAPIReports(reports []coremodels.Report) []models.APIReport {
	apiReports := make([]models.APIReport, len(reports))
	for i, report := range reports {
		apiReports[i] = convertToAPIReport(report)
	}
	return apiReports
}
