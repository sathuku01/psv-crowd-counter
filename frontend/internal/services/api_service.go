package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"psv-crowd-counter/frontend/internal/models"
)

// APIService handles communication with the backend API
type APIService struct {
	baseURL        string
	mediaPipeURL   string
	crowdDetectURL string
	httpClient     *http.Client
}

// NewAPIService creates a new API service instance
func NewAPIService() *APIService {
	baseURL := os.Getenv("BACKEND_API_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080" // Default backend URL
	} else {
		// If BACKEND_API_URL is set from service discovery (just hostname),
		// construct the full URL with protocol and port
		if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
			baseURL = "http://" + baseURL + ":8080"
		}
	}

	mediaPipeURL := os.Getenv("MEDIAPIPE_URL")
	if mediaPipeURL == "" {
		mediaPipeURL = "http://localhost:5000" // Default MediaPipe URL
	}

	crowdDetectURL := os.Getenv("CROWD_DETECT_URL")
	if crowdDetectURL == "" {
		crowdDetectURL = "http://localhost:8081" // Default crowd detection URL
	}

	return &APIService{
		baseURL:        baseURL,
		mediaPipeURL:   mediaPipeURL,
		crowdDetectURL: crowdDetectURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetMediaPipeURL returns the MediaPipe server URL
func (s *APIService) GetMediaPipeURL() string {
	return s.mediaPipeURL
}

// GetCrowdDetectURL returns the crowd detection server URL
func (s *APIService) GetCrowdDetectURL() string {
	return s.crowdDetectURL
}

// GetReports fetches all reports from the backend with retry logic
func (s *APIService) GetReports() ([]models.Report, error) {
	url := fmt.Sprintf("%s/reports", s.baseURL)

	var lastError error
	// Try up to 3 times with exponential backoff
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := s.httpClient.Get(url)
		if err != nil {
			lastError = fmt.Errorf("failed to fetch reports (attempt %d): %w", attempt+1, err)
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastError = fmt.Errorf("backend returned status %d (attempt %d)", resp.StatusCode, attempt+1)
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastError = fmt.Errorf("failed to read response body (attempt %d): %w", attempt+1, err)
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}

		// API returns wrapped response: {"success": true, "data": [...], "meta": {...}}
		var apiResponse struct {
			Success bool            `json:"success"`
			Data    json.RawMessage `json:"data"`
			Error   *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error,omitempty"`
		}

		if err := json.Unmarshal(body, &apiResponse); err != nil {
			lastError = fmt.Errorf("failed to unmarshal API response (attempt %d): %w", attempt+1, err)
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}

		if !apiResponse.Success {
			if apiResponse.Error != nil {
				lastError = fmt.Errorf("API error: %s - %s (attempt %d)", apiResponse.Error.Code, apiResponse.Error.Message, attempt+1)
			} else {
				lastError = fmt.Errorf("API returned unsuccessful response (attempt %d)", attempt+1)
			}
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}

		var reports []models.Report
		if err := json.Unmarshal(apiResponse.Data, &reports); err != nil {
			lastError = fmt.Errorf("failed to unmarshal reports (attempt %d): %w", attempt+1, err)
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}

		// Success! Return the reports
		return reports, nil
	}

	// All attempts failed
	return nil, lastError
}

// GetHealth checks the backend health status with retry logic
func (s *APIService) GetHealth() (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/health", s.baseURL)

	var lastError error
	// Try up to 3 times with exponential backoff
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := s.httpClient.Get(url)
		if err != nil {
			lastError = fmt.Errorf("failed to check health (attempt %d): %w", attempt+1, err)
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastError = fmt.Errorf("backend returned status %d (attempt %d)", resp.StatusCode, attempt+1)
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastError = fmt.Errorf("failed to read response body (attempt %d): %w", attempt+1, err)
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}

		// API returns wrapped response: {"success": true, "data": {...}, "meta": {...}}
		var apiResponse struct {
			Success bool            `json:"success"`
			Data    json.RawMessage `json:"data"`
			Error   *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error,omitempty"`
		}

		if err := json.Unmarshal(body, &apiResponse); err != nil {
			lastError = fmt.Errorf("failed to unmarshal API response (attempt %d): %w", attempt+1, err)
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}

		if !apiResponse.Success {
			if apiResponse.Error != nil {
				lastError = fmt.Errorf("API error: %s - %s (attempt %d)", apiResponse.Error.Code, apiResponse.Error.Message, attempt+1)
			} else {
				lastError = fmt.Errorf("API returned unsuccessful response (attempt %d)", attempt+1)
			}
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}

		var health map[string]interface{}
		if err := json.Unmarshal(apiResponse.Data, &health); err != nil {
			lastError = fmt.Errorf("failed to unmarshal health (attempt %d): %w", attempt+1, err)
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}

		// Success! Return the health
		return health, nil
	}

	// All attempts failed
	return nil, lastError
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
		totalPassengers += report.PassengerCount

		if _, exists := vehicleMap[report.BusID]; !exists {
			vehicleMap[report.BusID] = &models.VehicleStatus{
				BusID:       report.BusID,
				LastUpdated: report.Timestamp,
			}
		}

		// Update with latest report
		if report.Timestamp.After(vehicleMap[report.BusID].LastUpdated) {
			vehicleMap[report.BusID].PassengerCount = report.PassengerCount
			vehicleMap[report.BusID].LastUpdated = report.Timestamp
		}
	}

	// Convert map to slice and determine status
	vehicles := make([]models.VehicleStatus, 0, len(vehicleMap))
	activeCount := 0
	for _, v := range vehicleMap {
		// Determine status based on recent activity
		timeSinceUpdate := time.Since(v.LastUpdated)
		if timeSinceUpdate > 5*time.Minute {
			v.Status = "inactive"
		} else {
			v.Status = "active"
			activeCount++
		}
		vehicles = append(vehicles, *v)
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
		Vehicles:        vehicles,
		RecentReports:   recentReports,
		LastUpdated:     time.Now(),
	}, nil
}

// GetAnalyticsData fetches and processes analytics data from the backend with retry logic
func (s *APIService) GetAnalyticsData() (*models.AnalyticsData, error) {
	url := fmt.Sprintf("%s/api/v1/analytics", s.baseURL)

	var lastError error
	// Try up to 3 times with exponential backoff
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := s.httpClient.Get(url)
		if err != nil {
			lastError = fmt.Errorf("failed to fetch analytics (attempt %d): %w", attempt+1, err)
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastError = fmt.Errorf("backend returned status %d (attempt %d)", resp.StatusCode, attempt+1)
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastError = fmt.Errorf("failed to read response body (attempt %d): %w", attempt+1, err)
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}

		// API returns wrapped response: {"success": true, "data": {...}, "meta": {...}}
		var apiResponse struct {
			Success bool            `json:"success"`
			Data    json.RawMessage `json:"data"`
			Error   *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error,omitempty"`
		}

		if err := json.Unmarshal(body, &apiResponse); err != nil {
			lastError = fmt.Errorf("failed to unmarshal API response (attempt %d): %w", attempt+1, err)
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}

		if !apiResponse.Success {
			if apiResponse.Error != nil {
				lastError = fmt.Errorf("API error: %s - %s (attempt %d)", apiResponse.Error.Code, apiResponse.Error.Message, attempt+1)
			} else {
				lastError = fmt.Errorf("API returned unsuccessful response (attempt %d)", attempt+1)
			}
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}

		var analytics models.AnalyticsData
		if err := json.Unmarshal(apiResponse.Data, &analytics); err != nil {
			lastError = fmt.Errorf("failed to unmarshal analytics (attempt %d): %w", attempt+1, err)
			// Wait before retrying (except on last attempt)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}

		// Calculate max hourly count for chart scaling
		maxCount := 0
		for _, count := range analytics.HourlyDistribution {
			if count > maxCount {
				maxCount = count
			}
		}
		analytics.MaxHourlyCount = maxCount

		// Get recent reports (last 10)
		reports, err := s.GetReports()
		if err == nil && len(reports) > 0 {
			recentReports := reports
			if len(recentReports) > 10 {
				recentReports = recentReports[len(recentReports)-10:]
			}
			analytics.RecentReports = recentReports
		}

		// Set last updated time
		analytics.LastUpdated = time.Now()

		// Success! Return the analytics
		return &analytics, nil
	}

	// All attempts failed
	return nil, lastError
}
