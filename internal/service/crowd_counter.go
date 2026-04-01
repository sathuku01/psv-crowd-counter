package service

import (
	"image"
	"image/color"
	"math"
	"sort"
	"time"

	"gocv.io/x/gocv"

	"psv-crowd-counter/internal/core/models"
	"psv-crowd-counter/internal/core/ports"
)

// CrowdCounterService implements the CrowdCounter interface for head-based crowd counting
type CrowdCounterService struct {
	config  ports.CrowdCounterConfig
	tracker *HeadTracker
}

// HeadTracker maintains persistent IDs for head detections
type HeadTracker struct {
	nextID       int
	tracks       map[int]*models.HeadDetection
	maxAge       int
	iouThreshold float32
}

// Config holds crowd counter configuration
type Config struct {
	ConfidenceThreshold float32
	NMSThreshold        float32
	HeadHeightRatio     float32
	MinHeadSize         int
	MaxHeadSize         int
	MaxTrackAge         int
	TrackIOUThreshold   float32
}

// DefaultConfig returns default crowd counter configuration
func DefaultConfig() Config {
	return Config{
		ConfidenceThreshold: 0.20,
		NMSThreshold:        0.35,
		HeadHeightRatio:     0.25,
		MinHeadSize:         15,
		MaxHeadSize:         200,
		MaxTrackAge:         10,
		TrackIOUThreshold:   0.3,
	}
}

// NewCrowdCounterService creates a new crowd counter service
func NewCrowdCounterService(cfg Config) *CrowdCounterService {
	return &CrowdCounterService{
		config: ports.CrowdCounterConfig{
			ConfidenceThreshold: cfg.ConfidenceThreshold,
			NMSThreshold:        cfg.NMSThreshold,
			HeadHeightRatio:     cfg.HeadHeightRatio,
			MinHeadSize:         cfg.MinHeadSize,
			MaxHeadSize:         cfg.MaxHeadSize,
			MaxTrackAge:         cfg.MaxTrackAge,
			TrackIOUThreshold:   cfg.TrackIOUThreshold,
		},
		tracker: &HeadTracker{
			nextID:       1,
			tracks:       make(map[int]*models.HeadDetection),
			maxAge:       cfg.MaxTrackAge,
			iouThreshold: cfg.TrackIOUThreshold,
		},
	}
}

// ProcessFrame processes a frame and returns crowd count results
// Implements ports.CrowdCounter interface
func (c *CrowdCounterService) ProcessFrame(frame gocv.Mat) *models.CrowdCountResult {
	result := &models.CrowdCountResult{
		Timestamp: time.Now(),
	}

	if frame.Empty() {
		return result
	}

	// Placeholder - actual detection should be done by the detector
	// This method expects pre-filtered person detections
	result.PersonCount = 0
	result.HeadCount = 0

	return result
}

// ProcessWithDetections processes frame with pre-detected persons
func (c *CrowdCounterService) ProcessWithDetections(frame gocv.Mat, personDetections []models.Detection) *models.CrowdCountResult {
	result := &models.CrowdCountResult{
		Timestamp:     time.Now(),
		PersonDetects: personDetections,
	}

	if frame.Empty() || len(personDetections) == 0 {
		return result
	}

	result.PersonCount = len(personDetections)

	// Extract head regions from person detections
	headDetections := c.extractHeads(personDetections)
	result.HeadDetects = headDetections

	// Apply head NMS with lower threshold
	finalHeads := c.applyNMS(headDetections, 0.30)

	// Update tracker
	trackedHeads := c.tracker.Update(finalHeads)
	result.TrackedHeads = trackedHeads

	result.HeadCount = len(trackedHeads)

	return result
}

// extractHeads extracts head regions from person bounding boxes
func (c *CrowdCounterService) extractHeads(personDetections []models.Detection) []models.Detection {
	heads := []models.Detection{}

	for _, det := range personDetections {
		box := det.Box
		boxHeight := box.Max.Y - box.Min.Y
		boxWidth := box.Max.X - box.Min.X

		// Calculate head region (top portion of person bounding box)
		headHeight := int(float32(boxHeight) * c.config.HeadHeightRatio)

		// Ensure minimum head height
		if headHeight < c.config.MinHeadSize/2 {
			headHeight = c.config.MinHeadSize / 2
		}

		// Create head bounding box
		headTop := box.Min.Y
		headBottom := box.Min.Y + headHeight

		// Expand head width slightly (to capture more of the head)
		headLeft := box.Min.X - int(float32(boxWidth)*0.1)
		headRight := box.Max.X + int(float32(boxWidth)*0.1)

		// Clamp to person box bounds
		if headLeft < box.Min.X {
			headLeft = box.Min.X
		}
		if headRight > box.Max.X {
			headRight = box.Max.X
		}

		headBox := image.Rect(headLeft, headTop, headRight, headBottom)

		// Validate head box size
		headWidth := headBox.Max.X - headBox.Min.X
		headHeightActual := headBox.Max.Y - headBox.Min.Y

		if headWidth >= c.config.MinHeadSize && headHeightActual >= c.config.MinHeadSize &&
			headWidth <= c.config.MaxHeadSize && headHeightActual <= c.config.MaxHeadSize {
			heads = append(heads, models.Detection{
				Box:   headBox,
				Score: det.Score,
				Class: 0,
			})
		}
	}

	return heads
}

// applyNMS applies Non-Maximum Suppression to detections
func (c *CrowdCounterService) applyNMS(detections []models.Detection, iouThreshold float32) []models.Detection {
	if len(detections) == 0 {
		return detections
	}

	// Sort by score descending
	sort.Slice(detections, func(i, j int) bool {
		return detections[i].Score > detections[j].Score
	})

	result := []models.Detection{}
	used := make([]bool, len(detections))

	for i := 0; i < len(detections); i++ {
		if used[i] {
			continue
		}

		result = append(result, detections[i])

		for j := i + 1; j < len(detections); j++ {
			if used[j] {
				continue
			}

			if calcIOU(detections[i].Box, detections[j].Box) > iouThreshold {
				used[j] = true
			}
		}
	}

	return result
}

// GetConfig returns current configuration
// Implements ports.CrowdCounter interface
func (c *CrowdCounterService) GetConfig() ports.CrowdCounterConfig {
	return c.config
}

// Update matches new detections with existing tracks and creates new tracks
func (t *HeadTracker) Update(detections []models.Detection) []models.HeadDetection {
	now := time.Now()
	currentDetections := make([]*models.HeadDetection, 0, len(detections))
	used := make([]bool, len(detections))

	// Match existing tracks with new detections
	for _, track := range t.tracks {
		bestIdx := -1
		bestIOU := float32(0)

		for i, det := range detections {
			if used[i] {
				continue
			}
			// Calculate IOU between track and detection
			iouValue := calcIOU(track.Box, det.Box)
			if iouValue > bestIOU && iouValue > t.iouThreshold {
				bestIOU = iouValue
				bestIdx = i
			}
		}

		if bestIdx >= 0 {
			// Update existing track
			det := detections[bestIdx]
			track.Detection = det
			track.Centroid = image.Pt(
				(det.Box.Min.X+det.Box.Max.X)/2,
				(det.Box.Min.Y+det.Box.Max.Y)/2,
			)
			track.Age = 0
			track.LastSeen = now
			used[bestIdx] = true
			currentDetections = append(currentDetections, track)
		} else {
			// Track not matched - increment age
			track.Age++
			if track.Age <= t.maxAge {
				// Keep track alive for a few frames even without detection
				currentDetections = append(currentDetections, track)
			}
		}
	}

	// Create new tracks for unmatched detections
	for i, det := range detections {
		if !used[i] {
			track := &models.HeadDetection{
				Detection: det,
				ID:        t.nextID,
				Centroid: image.Pt(
					(det.Box.Min.X+det.Box.Max.X)/2,
					(det.Box.Min.Y+det.Box.Max.Y)/2,
				),
				Age:      0,
				LastSeen: now,
			}
			t.nextID++
			t.tracks[track.ID] = track
			currentDetections = append(currentDetections, track)
		}
	}

	// Clean up old tracks
	idsToDelete := make([]int, 0)
	for _, track := range t.tracks {
		if track.Age > t.maxAge {
			idsToDelete = append(idsToDelete, track.ID)
		}
	}
	for _, id := range idsToDelete {
		delete(t.tracks, id)
	}

	return dereferenceHeads(currentDetections)
}

// GetActiveCount returns the number of active (recently seen) tracks
func (t *HeadTracker) GetActiveCount() int {
	count := 0
	for _, track := range t.tracks {
		if track.Age <= t.maxAge {
			count++
		}
	}
	return count
}

// dereferenceHeads converts pointer slice to value slice
func dereferenceHeads(ptrs []*models.HeadDetection) []models.HeadDetection {
	result := make([]models.HeadDetection, 0, len(ptrs))
	for _, ptr := range ptrs {
		result = append(result, *ptr)
	}
	return result
}

// calcIOU calculates Intersection over Union between two rectangles
func calcIOU(a, b image.Rectangle) float32 {
	x1 := float32(maxInt(a.Min.X, b.Min.X))
	y1 := float32(maxInt(a.Min.Y, b.Min.Y))
	x2 := float32(minInt(a.Max.X, b.Max.X))
	y2 := float32(minInt(a.Max.Y, b.Max.Y))

	interWidth := float32(maxInt(0, int(x2-x1)))
	interHeight := float32(maxInt(0, int(y2-y1)))
	intersection := interWidth * interHeight

	if intersection == 0 {
		return 0
	}

	areaA := float32((a.Max.X - a.Min.X) * (a.Max.Y - a.Min.Y))
	areaB := float32((b.Max.X - b.Min.X) * (b.Max.Y - b.Min.Y))
	union := areaA + areaB - intersection

	return intersection / union
}

// centroidDistance calculates Euclidean distance between two points
func centroidDistance(a, b image.Point) float64 {
	dx := float64(a.X - b.X)
	dy := float64(a.Y - b.Y)
	return math.Sqrt(dx*dx + dy*dy)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Ensure CrowdCounterService implements ports.CrowdCounter
var _ ports.CrowdCounter = (*CrowdCounterService)(nil)

// ImageDrawer handles drawing detection results on frames
type ImageDrawer struct{}

// NewImageDrawer creates a new image drawer
func NewImageDrawer() *ImageDrawer {
	return &ImageDrawer{}
}

// DrawPersonDetections draws person bounding boxes on the image
func (d *ImageDrawer) DrawPersonDetections(frame *gocv.Mat, detections []models.Detection) {
	bodyColor := color.RGBA{255, 100, 0, 0} // Orange for bodies
	for _, det := range detections {
		gocv.Rectangle(frame, det.Box, bodyColor, 1)
	}
}

// DrawHeadDetections draws head bounding boxes on the image
func (d *ImageDrawer) DrawHeadDetections(frame *gocv.Mat, detections []models.Detection) {
	headColor := color.RGBA{0, 255, 255, 0} // Cyan for heads
	for _, det := range detections {
		gocv.Rectangle(frame, det.Box, headColor, 2)
	}
}

// DrawTrackedHeads draws tracked head information on the image
func (d *ImageDrawer) DrawTrackedHeads(frame *gocv.Mat, heads []models.HeadDetection) {
	headColor := color.RGBA{0, 255, 255, 0} // Cyan for heads
	white := color.RGBA{255, 255, 255, 0}
	black := color.RGBA{0, 0, 0, 0}

	for _, h := range heads {
		// Draw head rectangle
		gocv.Rectangle(frame, h.Box, headColor, 2)

		// Draw head ID
		label := "H" + string(rune('0'+h.ID%10))
		textSize := gocv.GetTextSize(label, gocv.FontHersheySimplex, 0.4, 1)

		textBg := image.Rect(
			h.Box.Min.X,
			h.Box.Min.Y-20,
			h.Box.Min.X+textSize.X+8,
			h.Box.Min.Y,
		)

		if textBg.Min.Y < 0 {
			textBg = image.Rect(
				h.Box.Min.X,
				h.Box.Min.Y,
				h.Box.Min.X+textSize.X+8,
				h.Box.Min.Y+20,
			)
		}

		gocv.Rectangle(frame, textBg, black, -1)

		textPt := image.Pt(h.Box.Min.X+4, h.Box.Min.Y-4)
		if textBg.Min.Y == h.Box.Min.Y {
			textPt = image.Pt(h.Box.Min.X+4, h.Box.Min.Y+14)
		}

		gocv.PutText(
			frame,
			label,
			textPt,
			gocv.FontHersheySimplex,
			0.4,
			white,
			1,
		)
	}
}

// DrawCount draws crowd count on the image
func (d *ImageDrawer) DrawCount(frame *gocv.Mat, headCount, personCount int) {
	label := "Heads: " + string(rune('0'+headCount%10))
	if headCount >= 10 {
		label = "Heads: " + string(rune('0'+(headCount/10)%10)) + string(rune('0'+headCount%10))
	}
	if headCount >= 100 {
		label = "Heads: 99+"
	}

	green := color.RGBA{0, 255, 0, 0}
	gocv.PutText(
		frame,
		label,
		image.Pt(10, 30),
		gocv.FontHersheySimplex,
		0.8,
		green,
		2,
	)
}
