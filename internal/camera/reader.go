package camera

import (
	"image"
	"time"

	"gocv.io/x/gocv"
)

type VideoCamera struct {
	cap      *gocv.VideoCapture
	interval time.Duration
}

func NewVideoCamera(videoPath string, interval time.Duration) *VideoCamera {
	cap, err := gocv.VideoCaptureFile(videoPath)
	if err != nil {
		panic(err)
	}
	return &VideoCamera{
		cap:      cap,
		interval: interval,
	}
}

func (v *VideoCamera) GetFrame() (image.Image, error) {
	img := gocv.NewMat()
	if ok := v.cap.Read(&img); !ok {
		return nil, nil // end of video
	}
	if img.Empty() {
		return nil, nil
	}
	return img.ToImage()
}

func (v *VideoCamera) Close() {
	v.cap.Close()
}
