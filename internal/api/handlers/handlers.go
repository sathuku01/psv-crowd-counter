package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"psv-crowd-counter/internal/api/models"
	coremodels "psv-crowd-counter/internal/core/models"
	"psv-crowd-counter/internal/detector"
	"psv-crowd-counter/internal/service"
	"psv-crowd-counter/internal/storage"

	"gocv.io/x/gocv"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// Handler holds dependencies for API handlers
type Handler struct {
	store      storage.Store
	processor  *service.Processor
	startTime  time.Time
	detections chan detector.Result
}

// NewHandler creates a new Handler instance
func NewHandler(store storage.Store, processor *service.Processor, detections chan detector.Result) *Handler {
	return &Handler{
		store:      store,
		processor:  processor,
		startTime:  time.Now(),
		detections: detections,
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
			totalPassengers := report.PassengerCount
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
		Timestamp:      time.Now().UTC(),
		BusID:          req.BusID,
		PassengerCount: req.PassengerCount,
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

// LiveDetections handles WebSocket connections for real-time detection results
func (h *Handler) LiveDetections(w http.ResponseWriter, r *http.Request) {
	// Get mode from query parameter (sleep or crowd)
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "sleep"
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionContextTakeover,
	})
	if err != nil {
		log.Printf("WebSocket accept error: %v", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "Connection closed")

	log.Printf("WebSocket connected: mode=%s", mode)

	for {
		// Read message from client (contains base64 image)
		var msg map[string]interface{}
		err := wsjson.Read(r.Context(), conn, &msg)
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			return
		}

		// Handle ping/pong
		if msgType, ok := msg["type"].(string); ok && msgType == "ping" {
			wsjson.Write(r.Context(), conn, map[string]string{"type": "pong"})
			continue
		}

		// Get image data
		imageB64, ok := msg["image"].(string)
		if !ok {
			continue
		}

		// Process detection
		result := h.processImageForDetection(imageB64, mode)
		if result == nil {
			continue
		}

		// Send results back to client
		if err := wsjson.Write(r.Context(), conn, result); err != nil {
			log.Printf("WebSocket write error: %v", err)
			return
		}
	}
}

// processImageForDetection processes a base64 image and returns detection results
// mode parameter determines detection type: "sleep" for face/landmark detection, "crowd" for people detection
func (h *Handler) processImageForDetection(imageB64 string, mode string) map[string]interface{} {
	// Decode base64 image
	imgData, err := base64.StdEncoding.DecodeString(imageB64)
	if err != nil {
		// Try URL encoding
		imgData, err = base64.URLEncoding.DecodeString(imageB64)
		if err != nil {
			log.Printf("Failed to decode base64: %v", err)
			return nil
		}
	}

	// Decode JPEG - IMDecode returns image and error
	img, err := gocv.IMDecode(imgData, gocv.IMReadColor)
	if err != nil {
		log.Printf("Failed to decode image: %v", err)
		return nil
	}
	if img.Empty() {
		log.Printf("Empty image")
		return nil
	}
	defer img.Close()

	if mode == "sleep" {
		// For sleep mode, delegate to MediaPipe server for facial landmark detection
		return h.callMediaPipeForDetection(imgData)
	} else if mode == "crowd" {
		// For crowd mode, use YOLOv8 ONNX model for people detection
		return h.processImageForCrowdDetection(img)
	} else {
		// Default to crowd detection if mode is unknown
		return h.processImageForCrowdDetection(img)
	}
}

// callMediaPipeForDetection delegates to the MediaPipe server for facial landmark detection
func (h *Handler) callMediaPipeForDetection(imgData []byte) map[string]interface{} {
	// Get MediaPipe URL from environment or use default
	mediaPipeURL := os.Getenv("MEDIAPIPE_URL")
	if mediaPipeURL == "" {
		mediaPipeURL = "http://localhost:5000" // Default MediaPipe URL
	}

	// Encode image to base64 for sending to MediaPipe server
	imageB64 := base64.StdEncoding.EncodeToString(imgData)

	// Create HTTP request to MediaPipe server
	payload := map[string]string{
		"image": imageB64,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal payload for MediaPipe: %v", err)
		return nil
	}

	resp, err := http.Post(mediaPipeURL+"/detect", "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		log.Printf("Failed to call MediaPipe server: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("MediaPipe server returned non-OK status: %v", resp.StatusCode)
		return nil
	}

	// Parse response from MediaPipe server
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Failed to decode MediaPipe response: %v", err)
		return nil
	}

	// Convert MediaPipe response to expected format
	// MediaPipe returns: face_detected, ear, mar, pitch, yaw, roll, face_box, left_eye_box, right_eye_box, mouth_box
	// We need to convert to: count, boxes (with type: "face" or "person" etc.)

	boxes := make([]map[string]interface{}, 0)

	// Add face box if face detected
	if faceDetected, ok := result["face_detected"].(bool); ok && faceDetected {
		if faceBox, ok := result["face_box"].([]interface{}); ok && len(faceBox) == 4 {
			boxes = append(boxes, map[string]interface{}{
				"x1":   int(faceBox[0].(float64)),
				"y1":   int(faceBox[1].(float64)),
				"x2":   int(faceBox[2].(float64)),
				"y2":   int(faceBox[3].(float64)),
				"type": "face",
			})
		}

		// Add left eye box
		if leftEyeBox, ok := result["left_eye_box"].([]interface{}); ok && len(leftEyeBox) == 4 {
			boxes = append(boxes, map[string]interface{}{
				"x1":   int(leftEyeBox[0].(float64)),
				"y1":   int(leftEyeBox[1].(float64)),
				"x2":   int(leftEyeBox[2].(float64)),
				"y2":   int(leftEyeBox[3].(float64)),
				"type": "left_eye",
			})
		}

		// Add right eye box
		if rightEyeBox, ok := result["right_eye_box"].([]interface{}); ok && len(rightEyeBox) == 4 {
			boxes = append(boxes, map[string]interface{}{
				"x1":   int(rightEyeBox[0].(float64)),
				"y1":   int(rightEyeBox[1].(float64)),
				"x2":   int(rightEyeBox[2].(float64)),
				"y2":   int(rightEyeBox[3].(float64)),
				"type": "right_eye",
			})
		}

		// Add mouth box
		if mouthBox, ok := result["mouth_box"].([]interface{}); ok && len(mouthBox) == 4 {
			boxes = append(boxes, map[string]interface{}{
				"x1":   int(mouthBox[0].(float64)),
				"y1":   int(mouthBox[1].(float64)),
				"x2":   int(mouthBox[2].(float64)),
				"y2":   int(mouthBox[3].(float64)),
				"type": "mouth",
			})
		}
	}

	resultMap := map[string]interface{}{
		"count": len(boxes),
		"boxes": boxes,
		// Also include the raw metrics for potential use by frontend
		"ear":           result["ear"],
		"mar":           result["mar"],
		"pitch":         result["pitch"],
		"yaw":           result["yaw"],
		"roll":          result["roll"],
		"face_detected": result["face_detected"],
	}

	return resultMap
}

// processImageForCrowdDetection processes an image using YOLOv8 ONNX model for people detection
func (h *Handler) processImageForCrowdDetection(img gocv.Mat) map[string]interface{} {
	// YOLOv8 ONNX path
	modelPath := "/home/sathuku/psv-crowd-counter/internal/core/models/yolov8n.onnx"

	// Load YOLOv8 ONNX model
	net := gocv.ReadNet(modelPath, "")
	if net.Empty() {
		log.Printf("Failed to load YOLOv8 ONNX model: %v", modelPath)
		return nil
	}
	defer net.Close()

	// Set preferable backend and target
	net.SetPreferableBackend(gocv.NetBackendDefault)
	net.SetPreferableTarget(gocv.NetTargetCPU)

	// Get image dimensions
	width := img.Cols()
	height := img.Rows()
	inputSize := 640               // YOLOv8 input size
	confThreshold := float32(0.25) // Confidence threshold
	nmsThreshold := float32(0.45)  // NMS threshold

	// Prepare input blob
	blob := gocv.BlobFromImage(
		img,
		1.0/255.0,
		image.Pt(inputSize, inputSize),
		gocv.NewScalar(0, 0, 0, 0),
		true,  // swapRB (BGR to RGB)
		false, // crop
	)
	defer blob.Close()

	net.SetInput(blob, "")
	output := net.Forward("")
	defer output.Close()

	// Get output data
	data, err := output.DataPtrFloat32()
	if err != nil {
		log.Printf("Error getting data pointer: %v", err)
		return nil
	}

	// Get output shape
	size := output.Size()
	if len(size) == 0 {
		log.Printf("Invalid output shape")
		return nil
	}

	// YOLOv8 output format handling
	var detections []image.Rectangle

	if len(size) == 3 {
		// Standard YOLOv8 output: [1, num_classes+4, num_predictions]
		// where num_classes = 80 for COCO, +4 for bbox coordinates
		numPredictions := size[2] // Usually 8400
		numChannels := size[1]    // Usually 84 (4 bbox + 80 classes)

		// Calculate scaling factors
		scaleX := float32(width) / float32(inputSize)
		scaleY := float32(height) / float32(inputSize)

		// For each prediction
		for i := 0; i < numPredictions; i++ {
			// Find the best class score
			bestClass := -1
			bestScore := float32(0)

			// Class scores start at index 4 (after bbox coordinates)
			for j := 4; j < numChannels; j++ {
				// Access: data[channel * numPredictions + prediction]
				idx := j*numPredictions + i
				if idx >= len(data) {
					continue
				}
				score := data[idx]
				if score > bestScore {
					bestScore = score
					bestClass = j - 4
				}
			}

			// Check if it's a person (class 0) with sufficient confidence
			if bestClass == 0 && bestScore > confThreshold {
				// Get bbox coordinates
				cxIdx := 0*numPredictions + i
				cyIdx := 1*numPredictions + i
				wIdx := 2*numPredictions + i
				hIdx := 3*numPredictions + i

				if cxIdx >= len(data) || cyIdx >= len(data) || wIdx >= len(data) || hIdx >= len(data) {
					continue
				}

				cx := data[cxIdx]
				cy := data[cyIdx]
				w := data[wIdx]
				h := data[hIdx]

				// Convert to pixel coordinates
				left := int((cx - w/2) * scaleX)
				top := int((cy - h/2) * scaleY)
				right := int((cx + w/2) * scaleX)
				bottom := int((cy + h/2) * scaleY)

				// Clamp to image bounds
				left = max(0, min(left, width))
				top = max(0, min(top, height))
				right = max(0, min(right, width))
				bottom = max(0, min(bottom, height))

				// Filter out invalid boxes
				if right > left && bottom > top {
					detections = append(detections, image.Rect(left, top, right, bottom))
				}
			}
		}
	} else {
		// Alternative format: [1, num_predictions, num_channels]
		numPredictions := size[1]
		numChannels := size[2]

		if numPredictions == 0 || numChannels == 0 {
			log.Printf("Invalid output dimensions")
			return nil
		}

		scaleX := float32(width) / float32(inputSize)
		scaleY := float32(height) / float32(inputSize)

		for i := 0; i < numPredictions; i++ {
			baseIdx := i * numChannels
			if baseIdx+numChannels > len(data) {
				continue
			}

			// Get bbox coordinates
			cx := data[baseIdx]
			cy := data[baseIdx+1]
			w := data[baseIdx+2]
			h := data[baseIdx+3]

			// Find best class
			bestClass := -1
			bestScore := float32(0)

			for j := 4; j < numChannels; j++ {
				score := data[baseIdx+j]
				if score > bestScore {
					bestScore = score
					bestClass = j - 4
				}
			}

			if bestClass == 0 && bestScore > confThreshold {
				left := int((cx - w/2) * scaleX)
				top := int((cy - h/2) * scaleY)
				right := int((cx + w/2) * scaleX)
				bottom := int((cy + h/2) * scaleY)

				left = max(0, min(left, width))
				top = max(0, min(top, height))
				right = max(0, min(right, width))
				bottom = max(0, min(bottom, height))

				if right > left && bottom > top {
					detections = append(detections, image.Rect(left, top, right, bottom))
				}
			}
		}
	}

	// Apply Non-Maximum Suppression
	finalDetections := nms(detections, nmsThreshold)

	// Convert to boxes format
	boxes := make([]map[string]interface{}, len(finalDetections))
	for i, det := range finalDetections {
		boxes[i] = map[string]interface{}{
			"x1":   det.Min.X,
			"y1":   det.Min.Y,
			"x2":   det.Max.X,
			"y2":   det.Max.Y,
			"type": "person",
		}
	}

	result := map[string]interface{}{
		"count": len(finalDetections),
		"boxes": boxes,
	}

	log.Printf("Detection result: %d people found", len(finalDetections))

	return result
}

// Helper functions

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Non-Maximum Suppression
func nms(detections []image.Rectangle, iouThreshold float32) []image.Rectangle {
	if len(detections) == 0 {
		return detections
	}

	// Create detections with scores (assuming all have same score for simplicity)
	type detection struct {
		rect  image.Rectangle
		score float32
	}
	var detectionsWithScore []detection
	for _, rect := range detections {
		detectionsWithScore = append(detectionsWithScore, detection{rect: rect, score: 1.0})
	}

	// Sort by score descending
	sort.Slice(detectionsWithScore, func(i, j int) bool {
		return detectionsWithScore[i].score > detectionsWithScore[j].score
	})

	result := []image.Rectangle{}
	used := make([]bool, len(detectionsWithScore))

	for i := 0; i < len(detectionsWithScore); i++ {
		if used[i] {
			continue
		}

		result = append(result, detectionsWithScore[i].rect)

		for j := i + 1; j < len(detectionsWithScore); j++ {
			if used[j] {
				continue
			}

			if iou(detectionsWithScore[i].rect, detectionsWithScore[j].rect) > iouThreshold {
				used[j] = true
			}
		}
	}

	return result
}

// Calculate Intersection over Union
func iou(a, b image.Rectangle) float32 {
	// Calculate intersection
	x1 := float32(max(a.Min.X, b.Min.X))
	y1 := float32(max(a.Min.Y, b.Min.Y))
	x2 := float32(min(a.Max.X, b.Max.X))
	y2 := float32(min(a.Max.Y, b.Max.Y))

	if x2 <= x1 || y2 <= y1 {
		return 0
	}

	intersection := (x2 - x1) * (y2 - y1)
	areaA := float32((a.Max.X - a.Min.X) * (a.Max.Y - a.Min.Y))
	areaB := float32((b.Max.X - b.Min.X) * (b.Max.Y - b.Min.Y))
	union := areaA + areaB - intersection

	return intersection / union
}

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
		passengers := report.PassengerCount
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

	if req.PassengerCount < 0 {
		errors = append(errors, models.ValidationError{Field: "passenger_count", Message: "Passenger count must be non-negative"})
	}

	return errors
}

func generateReportID(report coremodels.Report) string {
	return fmt.Sprintf("%s-%d", report.BusID, report.Timestamp.UnixNano())
}

func convertToAPIReport(report coremodels.Report) models.APIReport {
	return models.APIReport{
		ID:             generateReportID(report),
		BusID:          report.BusID,
		PassengerCount: report.PassengerCount,
		Timestamp:      report.Timestamp,
	}
}

func convertToAPIReports(reports []coremodels.Report) []models.APIReport {
	apiReports := make([]models.APIReport, len(reports))
	for i, report := range reports {
		apiReports[i] = convertToAPIReport(report)
	}
	return apiReports
}
