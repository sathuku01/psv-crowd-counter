package ports

import (
	"image"
	"image/color"

	"gocv.io/x/gocv"

	"psv-crowd-counter/internal/core/models"
)

// Detector defines the interface for object detection implementations
type Detector interface {
	// Detect performs object detection on the given frame
	// Returns slice of detections or error
	Detect(frame gocv.Mat) ([]models.Detection, error)

	// Close releases resources
	Close() error
}

// Camera defines the interface for video capture implementations
type Camera interface {
	// Read reads the next frame from the camera
	// Returns the frame or nil if no frame available
	Read() (gocv.Mat, error)

	// IsOpened returns true if camera is ready to read frames
	IsOpened() bool

	// Close releases camera resources
	Close() error
}

// HeadExtractor defines the interface for extracting head regions from detections
type HeadExtractor interface {
	// ExtractHeads extracts head regions from person detections
	ExtractHeads(personDetections []models.Detection) []models.Detection
}

// Tracker defines the interface for tracking detections across frames
type Tracker interface {
	// Update matches new detections with existing tracks
	Update(detections []models.Detection) []models.HeadDetection

	// GetActiveCount returns number of active tracked objects
	GetActiveCount() int
}

// NMSProcessor defines the interface for non-maximum suppression
type NMSProcessor interface {
	// ApplyNMS applies non-maximum suppression to detections
	ApplyNMS(detections []models.Detection, iouThreshold float32) []models.Detection
}

// CrowdCounter defines the interface for crowd counting services
type CrowdCounter interface {
	// ProcessFrame processes a frame and returns crowd count results
	ProcessFrame(frame gocv.Mat) *models.CrowdCountResult

	// GetConfig returns current configuration
	GetConfig() CrowdCounterConfig
}

// CrowdCounterConfig holds configuration for crowd counting
type CrowdCounterConfig struct {
	ConfidenceThreshold float32
	NMSThreshold        float32
	HeadHeightRatio     float32
	MinHeadSize         int
	MaxHeadSize         int
	MaxTrackAge         int
	TrackIOUThreshold   float32
}

// ImageProcessor defines the interface for image drawing operations
type ImageProcessor interface {
	// DrawDetections draws detection boxes on the image
	DrawDetections(frame *gocv.Mat, detections []models.Detection, color color.RGBA)

	// DrawText draws text on the image
	DrawText(frame *gocv.Mat, text string, pt image.Point, color color.RGBA)

	// DrawHeadTracking draws head tracking information
	DrawHeadTracking(frame *gocv.Mat, heads []models.HeadDetection)
}

// ILogger defines the logging interface
type ILogger interface {
	// Info logs informational messages
	Info(args ...interface{})

	// Infof logs formatted informational messages
	Infof(format string, args ...interface{})

	// Error logs error messages
	Error(args ...interface{})

	// Errorf logs formatted error messages
	Errorf(format string, args ...interface{})

	// Debug logs debug messages
	Debug(args ...interface{})

	// Debugf logs formatted debug messages
	Debugf(format string, args ...interface{})
}
