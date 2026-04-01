package main

import (
	"fmt"
	"log"

	"gocv.io/x/gocv"

	"psv-crowd-counter/internal/camera"
	"psv-crowd-counter/internal/detector"
	"psv-crowd-counter/internal/logger"
	"psv-crowd-counter/internal/service"
)

// Application struct holds all dependencies
type Application struct {
	logger       *logger.Logger
	camera       *camera.VideoCapture
	detector     *detector.YOLODetector
	crowdCounter *service.CrowdCounterService
	drawer       *service.ImageDrawer
	window       *gocv.Window
}

// New creates a new application instance with dependency injection
func New(videoPath, modelPath string) (*Application, error) {
	log := logger.New()

	// Initialize camera
	cam, err := camera.NewVideoCapture(videoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create camera: %w", err)
	}

	// Initialize detector with custom configuration
	detectorConfig := detector.Config{
		ModelPath:     modelPath,
		InputSize:     640,
		ConfThreshold: 0.20,
		NMSThreshold:  0.35,
		PersonClassID: 0,
	}
	det, err := detector.NewYOLODetector(detectorConfig)
	if err != nil {
		cam.Close()
		return nil, fmt.Errorf("failed to create detector: %w", err)
	}

	// Initialize crowd counter service
	crowdCounter := service.NewCrowdCounterService(service.DefaultConfig())

	// Initialize image drawer
	drawer := service.NewImageDrawer()

	// Create display window
	window := gocv.NewWindow("Head-Based Crowd Counting")

	return &Application{
		logger:       log,
		camera:       cam,
		detector:     det,
		crowdCounter: crowdCounter,
		drawer:       drawer,
		window:       window,
	}, nil
}

// Run starts the main processing loop
func (app *Application) Run() error {
	defer app.Cleanup()

	frameCount := 0

	app.logger.Info("Starting crowd counting application...")

	for {
		// Read frame from camera
		frame, err := app.camera.Read()
		if err != nil {
			app.logger.Error("Failed to read frame", err)
			continue
		}
		if frame.Empty() {
			break // End of video
		}

		frameCount++

		// Run detection on frame
		personDetections, err := app.detector.Detect(frame)
		if err != nil {
			app.logger.Error("Detection failed:", err)
			frame.Close()
			continue
		}

		// Process detections with crowd counter
		result := app.crowdCounter.ProcessWithDetections(frame, personDetections)

		// Draw visualization
		app.drawer.DrawPersonDetections(&frame, result.PersonDetects)
		app.drawer.DrawTrackedHeads(&frame, result.TrackedHeads)
		app.drawer.DrawCount(&frame, result.HeadCount, result.PersonCount)

		// Display frame
		app.window.IMShow(frame)
		if app.window.WaitKey(1) >= 0 {
			break
		}

		frame.Close()

		// Print stats periodically
		if frameCount%30 == 0 {
			app.logger.Infof("Frame %d: %d persons, %d heads tracked",
				frameCount, result.PersonCount, result.HeadCount)
		}
	}

	app.logger.Infof("Processing complete. Total frames: %d", frameCount)
	return nil
}

// Cleanup releases all resources
func (app *Application) Cleanup() {
	if app.window != nil {
		app.window.Close()
	}
	if app.detector != nil {
		app.detector.Close()
	}
	if app.camera != nil {
		app.camera.Close()
	}
}

func main() {
	// Configuration - these could come from config file or environment
	videoPath := "/home/sathuku/Downloads/Delhi Metro bus.mp4"
	modelPath := "/home/sathuku/psv-crowd-counter/internal/core/models/yolov8n.onnx"

	// Create and run application
	app, err := New(videoPath, modelPath)
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
