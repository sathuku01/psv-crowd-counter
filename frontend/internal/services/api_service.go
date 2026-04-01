package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"psv-crowd-counter/frontend/internal/models"
)

// APIService handles communication with the backend API
type APIService struct {
	baseURL    string
	httpClient *http.Client
}

// NewAPIService creates a new API service instance
func NewAPIService() *APIService {
	baseURL := os.Getenv("BACKEND_API_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080" // Default backend URL
	}

	return &APIService{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetReports fetches all reports from the backend
func (s *APIService) GetReports() ([]models.Report, error) {
	url := fmt.Sprintf("%s/reports", s.baseURL)

	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch reports: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var reports []models.Report
	if err := json.Unmarshal(body, &reports); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reports: %w", err)
	}

	return reports, nil
}

// GetHealth checks the backend health status
func (s *APIService) GetHealth() (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/health", s.baseURL)

	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to check health: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var health map[string]interface{}
	if err := json.Unmarshal(body, &health); err != nil {
		return nil, fmt.Errorf("failed to unmarshal health: %w", err)
	}

	return health, nil
}

// GetDashboardData fetches and processes data for the dashboard
func (s *APIService) GetDashboardData() (*models.DashboardData, error) {
	reports, err := s.GetReports()
	if err != nil {
		return nil, err
	}

	// Process reports to create dashboard data
	vehicleMap := make(map[string]*models.VehicleStatus)
	totalPassengers := 0

	for _, report := range reports {
		totalPassengers += report.Front + report.Rear

		if _, exists := vehicleMap[report.BusID]; !exists {
			vehicleMap[report.BusID] = &models.VehicleStatus{
				BusID:       report.BusID,
				LastUpdated: report.Timestamp,
				SpeedKPH:    report.SpeedKPH,
			}
		}

		// Update with latest report
		if report.Timestamp.After(vehicleMap[report.BusID].LastUpdated) {
			vehicleMap[report.BusID].PassengerCount = report.Front + report.Rear
			vehicleMap[report.BusID].SpeedKPH = report.SpeedKPH
			vehicleMap[report.BusID].LastUpdated = report.Timestamp
		}
	}

	// Convert map to slice and determine status
	vehicles := make([]models.VehicleStatus, 0, len(vehicleMap))
	activeCount := 0
	for _, v := range vehicleMap {
		// Determine status based on speed and recent activity
		timeSinceUpdate := time.Since(v.LastUpdated)
		if timeSinceUpdate > 5*time.Minute {
			v.Status = "inactive"
		} else if v.SpeedKPH < 1.0 {
			v.Status = "idle"
		} else {
			v.Status = "active"
			activeCount++
		}
		vehicles = append(vehicles, *v)
	}

	// Calculate average density
	averageDensity := 0.0
	if len(vehicles) > 0 {
		totalDensity := 0.0
		for _, v := range vehicles {
			totalDensity += float64(v.PassengerCount)
		}
		averageDensity = totalDensity / float64(len(vehicles))
	}

	// Get recent reports (last 10)
	recentReports := reports
	if len(recentReports) > 10 {
		recentReports = recentReports[len(recentReports)-10:]
	}

	return &models.DashboardData{
		TotalPassengers: totalPassengers,
		ActiveVehicles:  activeCount,
		TotalVehicles:   len(vehicles),
		AverageDensity:  averageDensity,
		Vehicles:        vehicles,
		RecentReports:   recentReports,
		LastUpdated:     time.Now(),
	}, nil
}

// GetAnalyticsData fetches and processes historical analytics data
func (s *APIService) GetAnalyticsData(filter *models.FilterParams) (*models.AnalyticsData, error) {
	reports, err := s.GetReports()
	if err != nil {
		return nil, err
	}

	// Filter reports if parameters provided
	filteredReports := s.filterReports(reports, filter)

	// Calculate hourly statistics
	hourlyMap := make(map[int]*models.HourlyStats)
	for _, report := range filteredReports {
		hour := report.Timestamp.Hour()
		if _, exists := hourlyMap[hour]; !exists {
			hourlyMap[hour] = &models.HourlyStats{
				Hour: hour,
			}
		}
		hourlyMap[hour].ReportCount++
		hourlyMap[hour].AverageCount += float64(report.Front + report.Rear)
		if report.Front+report.Rear > hourlyMap[hour].MaxCount {
			hourlyMap[hour].MaxCount = report.Front + report.Rear
		}
	}

	// Calculate averages
	hourlyData := make([]models.HourlyStats, 0, 24)
	for hour := 0; hour < 24; hour++ {
		if stats, exists := hourlyMap[hour]; exists {
			if stats.ReportCount > 0 {
				stats.AverageCount /= float64(stats.ReportCount)
			}
			hourlyData = append(hourlyData, *stats)
		} else {
			hourlyData = append(hourlyData, models.HourlyStats{Hour: hour})
		}
	}

	// Calculate daily statistics
	dailyMap := make(map[string]*models.DailyStats)
	for _, report := range filteredReports {
		date := report.Timestamp.Format("2006-01-02")
		if _, exists := dailyMap[date]; !exists {
			dailyMap[date] = &models.DailyStats{
				Date: date,
			}
		}
		dailyMap[date].TotalReports++
		dailyMap[date].AverageCount += float64(report.Front + report.Rear)
		if report.Front+report.Rear > dailyMap[date].MaxCount {
			dailyMap[date].MaxCount = report.Front + report.Rear
		}
	}

	// Calculate averages and convert to slice
	dailyData := make([]models.DailyStats, 0, len(dailyMap))
	for _, stats := range dailyMap {
		if stats.TotalReports > 0 {
			stats.AverageCount /= float64(stats.TotalReports)
		}
		dailyData = append(dailyData, *stats)
	}

	// Find peak hours (top 5)
	peakHours := s.findPeakHours(hourlyData)

	return &models.AnalyticsData{
		HourlyData: hourlyData,
		DailyData:  dailyData,
		PeakHours:  peakHours,
	}, nil
}

// filterReports filters reports based on provided parameters
func (s *APIService) filterReports(reports []models.Report, filter *models.FilterParams) []models.Report {
	if filter == nil {
		return reports
	}

	filtered := make([]models.Report, 0)
	for _, report := range reports {
		// Filter by vehicle ID
		if filter.VehicleID != "" && report.BusID != filter.VehicleID {
			continue
		}

		// Filter by time range
		if filter.StartTime != "" {
			startTime, err := time.Parse("2006-01-02T15:04", filter.StartTime)
			if err == nil && report.Timestamp.Before(startTime) {
				continue
			}
		}

		if filter.EndTime != "" {
			endTime, err := time.Parse("2006-01-02T15:04", filter.EndTime)
			if err == nil && report.Timestamp.After(endTime) {
				continue
			}
		}

		filtered = append(filtered, report)
	}

	return filtered
}

// findPeakHours finds the top 5 peak hours
func (s *APIService) findPeakHours(hourlyData []models.HourlyStats) []models.PeakHour {
	// Sort by average count (descending)
	sorted := make([]models.HourlyStats, len(hourlyData))
	copy(sorted, hourlyData)

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].AverageCount > sorted[i].AverageCount {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Get top 5
	peakHours := make([]models.PeakHour, 0, 5)
	for i := 0; i < 5 && i < len(sorted); i++ {
		if sorted[i].AverageCount > 0 {
			peakHours = append(peakHours, models.PeakHour{
				Hour:         sorted[i].Hour,
				AverageCount: sorted[i].AverageCount,
				Day:          "All Days", // Could be enhanced to show specific days
			})
		}
	}

	return peakHours
}
