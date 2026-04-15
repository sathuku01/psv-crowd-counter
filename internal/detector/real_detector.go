package detector

import (
	"bytes"
	"image"
	_ "image/jpeg"
	"log"

	"psv-crowd-counter/internal/camera"

	"gocv.io/x/gocv"
)

// RealDetector implements the Detector interface using OpenCV
type RealDetector struct {
	out        chan Result
	quit       chan struct{}
	hog        *gocv.HOGDescriptor
	classifier gocv.CascadeClassifier
}

// NewRealDetector creates a new real detector
func NewRealDetector() *RealDetector {
	// Initialize HOG descriptor for person detection
	hog := gocv.NewHOGDescriptor()
	hog.SetSVMDetector(gocv.HOGDefaultPeopleDetector())

	// Initialize Haar cascade for face detection as fallback
	classifier := gocv.NewCascadeClassifier()
	classifier.Load("internal/core/models/haarcascade_frontalface_default.xml")

	return &RealDetector{
		out:        make(chan Result, 10),
		quit:       make(chan struct{}),
		hog:        &hog,
		classifier: classifier,
	}
}

// Start initializes the detector
func (r *RealDetector) Start() {
	log.Println("RealDetector started with OpenCV person detection")
}

// Stop stops the detector
func (r *RealDetector) Stop() {
	close(r.quit)
	close(r.out)
	r.hog.Close()
	r.classifier.Close()
}

// Process processes frames and detects crowd counts using OpenCV
func (r *RealDetector) Process(in <-chan camera.Frame) <-chan Result {
	go func() {
		defer close(r.out)
		for frame := range in {
			detections, count := r.detectPeople(&frame)

			boxes := ConvertDetectionsToBoxes(detections)

			result := Result{
				Timestamp:  frame,
				Count:      count,
				Detections: detections,
				Boxes:      boxes,
			}

			select {
			case r.out <- result:
			case <-r.quit:
				return
			}
		}
	}()
	return r.out
}

// detectPeople uses OpenCV to detect people in the frame
func (r *RealDetector) detectPeople(frame *camera.Frame) ([]Detection, int) {
	// Decode the JPEG frame data
	reader := bytes.NewReader(frame.Payload)
	img, _, err := image.Decode(reader)
	if err != nil {
		log.Printf("Failed to decode frame: %v", err)
		return nil, 0
	}

	// Convert to gocv.Mat
	mat, err := gocv.ImageToMatRGB(img)
	if err != nil {
		log.Printf("Failed to convert image to mat: %v", err)
		return nil, 0
	}
	defer mat.Close()

	var detections []Detection

	// Try HOG person detection first
	rects := r.hog.DetectMultiScale(mat)
	for _, rect := range rects {
		detections = append(detections, Detection{
			X:      rect.Min.X,
			Y:      rect.Min.Y,
			Width:  rect.Max.X - rect.Min.X,
			Height: rect.Max.Y - rect.Min.Y,
			Type:   "person",
		})
	}
	personCount := len(rects)

	// If HOG didn't find people, try Haar cascade face detection as fallback
	if personCount == 0 {
		rects = r.classifier.DetectMultiScale(mat)
		for _, rect := range rects {
			detections = append(detections, Detection{
				X:      rect.Min.X,
				Y:      rect.Min.Y,
				Width:  rect.Max.X - rect.Min.X,
				Height: rect.Max.Y - rect.Min.Y,
				Type:   "face",
			})
		}
		// Estimate people count from faces (rough approximation)
		personCount = len(rects)
		if personCount > 0 {
			// Adjust for multiple faces per person and detection misses
			personCount = int(float64(personCount) * 1.2)
		}
	}

	// Ensure minimum count of 0
	if personCount < 0 {
		personCount = 0
	}

	return detections, personCount
}

// ConvertDetectionsToBoxes converts internal Detection format to frontend-compatible Box format
func ConvertDetectionsToBoxes(detections []Detection) []Box {
	boxes := make([]Box, len(detections))
	for i, d := range detections {
		boxes[i] = Box{
			X1:   d.X,
			Y1:   d.Y,
			X2:   d.X + d.Width,
			Y2:   d.Y + d.Height,
			Type: d.Type,
		}
	}
	return boxes
}
