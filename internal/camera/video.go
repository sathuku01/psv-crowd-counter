package camera

import (
	"fmt"

	"gocv.io/x/gocv"

	"psv-crowd-counter/internal/core/ports"
)

// VideoCapture implements ports.Camera interface using gocv.VideoCapture
type VideoCapture struct {
	cap *gocv.VideoCapture
}

// NewVideoCapture creates a new video capture instance
func NewVideoCapture(videoPath string) (*VideoCapture, error) {
	cap, err := gocv.VideoCaptureFile(videoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open video file: %w", err)
	}

	return &VideoCapture{cap: cap}, nil
}

// Read the next frame from the video
// Implements ports.Camera interface
func (v *VideoCapture) Read() (gocv.Mat, error) {
	frame := gocv.NewMat()
	if ok := v.cap.Read(&frame); !ok {
		frame.Close()
		return gocv.Mat{}, nil // End of video
	}
	if frame.Empty() {
		frame.Close()
		return gocv.Mat{}, nil
	}
	return frame, nil
}

// IsOpened returns true if video capture is ready to read frames
// Implements ports.Camera interface
func (v *VideoCapture) IsOpened() bool {
	return v.cap != nil && v.cap.IsOpened()
}

// Close releases video capture resources
// Implements ports.Camera interface
func (v *VideoCapture) Close() error {
	if v.cap != nil {
		v.cap.Close()
	}
	return nil
}

// GetFrameWidth returns the frame width
func (v *VideoCapture) GetFrameWidth() float64 {
	return v.cap.Get(gocv.VideoCaptureFrameWidth)
}

// GetFrameHeight returns the frame height
func (v *VideoCapture) GetFrameHeight() float64 {
	return v.cap.Get(gocv.VideoCaptureFrameHeight)
}

// Ensure VideoCapture implements ports.Camera
var _ ports.Camera = (*VideoCapture)(nil)
