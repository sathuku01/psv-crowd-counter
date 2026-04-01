package detector

import (
	"fmt"
	"image"
	"sort"

	"gocv.io/x/gocv"

	"psv-crowd-counter/internal/core/models"
	"psv-crowd-counter/internal/core/ports"
)

// YOLODetector implements the Detector interface using YOLOv8
type YOLODetector struct {
	net           gocv.Net
	inputSize     int
	confThreshold float32
	nmsThreshold  float32
	classNames    []string
	personClassID int
}

// Config holds YOLO detector configuration
type Config struct {
	ModelPath     string
	InputSize     int
	ConfThreshold float32
	NMSThreshold  float32
	PersonClassID int
}

// DefaultConfig returns default YOLO detector configuration
func DefaultConfig() Config {
	return Config{
		ModelPath:     "yolov8n.onnx",
		InputSize:     640,
		ConfThreshold: 0.20,
		NMSThreshold:  0.35,
		PersonClassID: 0,
	}
}

// NewYOLODetector creates a new YOLO detector
func NewYOLODetector(cfg Config) (*YOLODetector, error) {
	net := gocv.ReadNet(cfg.ModelPath, "")
	if net.Empty() {
		return nil, fmt.Errorf("failed to load YOLO model from %s", cfg.ModelPath)
	}

	net.SetPreferableBackend(gocv.NetBackendDefault)
	net.SetPreferableTarget(gocv.NetTargetCPU)

	// COCO class names
	classNames := []string{
		"person", "bicycle", "car", "motorcycle", "airplane", "bus", "train", "truck", "boat",
		"traffic light", "fire hydrant", "stop sign", "parking meter", "bench", "bird", "cat",
		"dog", "horse", "sheep", "cow", "elephant", "bear", "zebra", "giraffe", "backpack",
		"umbrella", "handbag", "tie", "suitcase", "frisbee", "skis", "snowboard", "sports ball",
		"kite", "baseball bat", "baseball glove", "skateboard", "surfboard", "tennis racket",
		"bottle", "wine glass", "cup", "fork", "knife", "spoon", "bowl", "banana", "apple",
		"sandwich", "orange", "broccoli", "carrot", "hot dog", "pizza", "donut", "cake", "chair",
		"couch", "potted plant", "bed", "dining table", "toilet", "tv", "laptop", "mouse",
		"remote", "keyboard", "cell phone", "microwave", "oven", "toaster", "sink", "refrigerator",
		"book", "clock", "vase", "scissors", "teddy bear", "hair drier", "toothbrush",
	}

	return &YOLODetector{
		net:           net,
		inputSize:     cfg.InputSize,
		confThreshold: cfg.ConfThreshold,
		nmsThreshold:  cfg.NMSThreshold,
		classNames:    classNames,
		personClassID: cfg.PersonClassID,
	}, nil
}

// Detect performs YOLO object detection on the given frame
// Implements ports.Detector interface
func (y *YOLODetector) Detect(frame gocv.Mat) ([]models.Detection, error) {
	if frame.Empty() {
		return nil, nil
	}

	// Prepare input blob
	blob := gocv.BlobFromImage(
		frame,
		1.0/255.0,
		image.Pt(y.inputSize, y.inputSize),
		gocv.NewScalar(0, 0, 0, 0),
		true,
		false,
	)
	defer blob.Close()

	y.net.SetInput(blob, "")
	output := y.net.Forward("")
	defer output.Close()

	// Parse detections
	return y.parseOutput(output, frame)
}

// Close releases detector resources
func (y *YOLODetector) Close() error {
	if !y.net.Empty() {
		y.net.Close()
	}
	return nil
}

// parseOutput parses YOLO network output into structured detections
func (y *YOLODetector) parseOutput(output gocv.Mat, frame gocv.Mat) ([]models.Detection, error) {
	data, err := output.DataPtrFloat32()
	if err != nil {
		return nil, err
	}

	size := output.Size()
	detections := []models.Detection{}

	scaleX := float32(frame.Cols()) / float32(y.inputSize)
	scaleY := float32(frame.Rows()) / float32(y.inputSize)

	// Handle different YOLO output formats
	if len(size) == 3 {
		// Format: [1, num_channels, num_predictions]
		numPredictions := size[2]
		numChannels := size[1]

		for i := 0; i < numPredictions; i++ {
			// Find best class
			bestClass := -1
			bestScore := float32(0)

			for j := 4; j < numChannels; j++ {
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

			// Filter for persons with sufficient confidence
			if bestClass == y.personClassID && bestScore > y.confThreshold {
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
				left = maxInt(0, minInt(left, frame.Cols()))
				top = maxInt(0, minInt(top, frame.Rows()))
				right = maxInt(0, minInt(right, frame.Cols()))
				bottom = maxInt(0, minInt(bottom, frame.Rows()))

				if right > left && bottom > top {
					detections = append(detections, models.Detection{
						Box:   image.Rect(left, top, right, bottom),
						Score: bestScore,
						Class: bestClass,
					})
				}
			}
		}
	} else if len(size) == 2 {
		// Alternative format: [1, num_predictions, num_channels]
		numPredictions := size[0]
		numChannels := size[1]

		for i := 0; i < numPredictions; i++ {
			baseIdx := i * numChannels
			if baseIdx+numChannels > len(data) {
				continue
			}

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

			if bestClass == y.personClassID && bestScore > y.confThreshold {
				left := int((cx - w/2) * scaleX)
				top := int((cy - h/2) * scaleY)
				right := int((cx + w/2) * scaleX)
				bottom := int((cy + h/2) * scaleY)

				left = maxInt(0, minInt(left, frame.Cols()))
				top = maxInt(0, minInt(top, frame.Rows()))
				right = maxInt(0, minInt(right, frame.Cols()))
				bottom = maxInt(0, minInt(bottom, frame.Rows()))

				if right > left && bottom > top {
					detections = append(detections, models.Detection{
						Box:   image.Rect(left, top, right, bottom),
						Score: bestScore,
						Class: bestClass,
					})
				}
			}
		}
	}

	// Apply NMS
	return y.applyNMS(detections), nil
}

// applyNMS applies Non-Maximum Suppression
func (y *YOLODetector) applyNMS(detections []models.Detection) []models.Detection {
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

			if calcIOU(detections[i].Box, detections[j].Box) > y.nmsThreshold {
				used[j] = true
			}
		}
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

// Ensure YOLODetector implements ports.Detector
var _ ports.Detector = (*YOLODetector)(nil)

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
