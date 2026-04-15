package camera

import (
	"log"
	"time"

	"gocv.io/x/gocv"
)

// RealCamera implements the Camera interface using gocv
type RealCamera struct {
	deviceID int
	interval time.Duration
	frames   chan Frame
	quit     chan struct{}
}

// NewRealCamera creates a new real camera that captures from a webcam device
func NewRealCamera(deviceID int, interval time.Duration) *RealCamera {
	return &RealCamera{
		deviceID: deviceID,
		interval: interval,
		frames:   make(chan Frame, 10),
		quit:     make(chan struct{}),
	}
}

// Start begins capturing frames from the webcam
func (r *RealCamera) Start() {
	go func() {
		cap, err := gocv.VideoCaptureDevice(r.deviceID)
		if err != nil {
			log.Printf("Failed to open camera device %d: %v", r.deviceID, err)
			close(r.frames)
			return
		}
		defer cap.Close()

		// Set camera properties for better performance
		cap.Set(gocv.VideoCaptureFrameWidth, 640)
		cap.Set(gocv.VideoCaptureFrameHeight, 480)
		cap.Set(gocv.VideoCaptureFPS, 10)

		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				img := gocv.NewMat()
				if ok := cap.Read(&img); !ok || img.Empty() {
					log.Println("Failed to read frame from camera or frame is empty")
					img.Close()
					continue
				}

				buf, err := gocv.IMEncode(".jpg", img)
				if err != nil {
					log.Printf("Failed to encode frame: %v", err)
					img.Close()
					continue
				}

				frame := Frame{
					Timestamp: time.Now(),
					Payload:   buf.GetBytes(),
				}

				select {
				case r.frames <- frame:
				default:
					// Drop frame if channel is full (critical for real-time)
				}

				img.Close()
				buf.Close()
			case <-r.quit:
				close(r.frames)
				return
			}
		}
	}()
}

// Stop stops the camera
func (r *RealCamera) Stop() {
	close(r.quit)
}

// Frames returns the channel of captured frames
func (r *RealCamera) Frames() <-chan Frame {
	return r.frames
}
