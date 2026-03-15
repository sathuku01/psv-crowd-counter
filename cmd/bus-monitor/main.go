package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"net/http"
	"os"
	"time"

	"gocv.io/x/gocv"
)

const (
	EARThreshold = 0.25 // Eye Aspect Ratio threshold for drowsiness
	MaxFrames    = 20   // Number of consecutive frames with closed eyes to trigger alert
	MediaPipeURL = "http://localhost:5000/detect"
)

var closedFrames int

// MediaPipeResponse represents the response from MediaPipe server
type MediaPipeResponse struct {
	FaceDetected bool    `json:"face_detected"`
	EAR          float64 `json:"ear"`
	LeftEAR      float64 `json:"left_ear"`
	RightEAR     float64 `json:"right_ear"`
	FaceBox      []int   `json:"face_box"`
	LeftEyeBox   []int   `json:"left_eye_box"`
	RightEyeBox  []int   `json:"right_eye_box"`
	MouthBox     []int   `json:"mouth_box"`
	LeftEye      [][]int `json:"left_eye"`
	RightEye     [][]int `json:"right_eye"`
	Error        string  `json:"error,omitempty"`
}

func main() {
	// Check if MediaPipe server is available
	if !checkMediaPipeServer() {
		fmt.Println("MediaPipe server not available, running with face detection only")
		runWithFaceDetection()
		return
	}

	fmt.Println("Connected to MediaPipe server")
	runWithMediaPipe()
}

func checkMediaPipeServer() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:5000/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func runWithFaceDetection() {
	webcam, err := openVideoCapture()
	if err != nil {
		fmt.Printf("Error opening camera: %v\n", err)
		runTestMode()
		return
	}
	defer webcam.Close()

	window := gocv.NewWindow("Driver Drowsiness Detection")
	defer window.Close()

	frame := gocv.NewMat()
	defer frame.Close()

	faceNet := gocv.ReadNetFromCaffe(
		"/home/sathuku/psv-crowd-counter/internal/core/models/deploy.prototxt",
		"/home/sathuku/psv-crowd-counter/internal/core/models/res10_300x300_ssd_iter_140000.caffemodel",
	)
	if faceNet.Empty() {
		fmt.Println("Failed to load face detector")
		return
	}
	defer faceNet.Close()

	fmt.Println("Running with face detection only")

	for {
		if ok := webcam.Read(&frame); !ok {
			continue
		}
		if frame.Empty() {
			continue
		}

		faces := detectFaces(frame, &faceNet)
		for _, face := range faces {
			gocv.Rectangle(&frame, face, color.RGBA{0, 255, 0, 0}, 2)
		}

		gocv.PutText(&frame, "Face Detection Mode", image.Pt(10, 30),
			gocv.FontHersheySimplex, 0.7, color.RGBA{255, 255, 255, 0}, 1)

		window.IMShow(frame)
		if window.WaitKey(1) == 27 {
			break
		}
	}
}

// openVideoCapture attempts to open camera or video file
func openVideoCapture() (*gocv.VideoCapture, error) {
	// First try video file if specified via environment variable or default path
	videoFile := os.Getenv("VIDEO_FILE")
	if videoFile == "" {
		// Check for default video file
		if _, err := os.Stat("/home/sathuku/psv-crowd-counter/internal/camera/mock/Science of sleep - Fatigue effects on driving 60sec.mp4"); err == nil {
			videoFile = "/home/sathuku/psv-crowd-counter/internal/camera/mock/Science of sleep - Fatigue effects on driving 60sec.mp4"
		}
	}

	if videoFile != "" {
		fmt.Printf("Opening video file: %s\n", videoFile)
		webcam, err := gocv.OpenVideoCapture(videoFile)
		if err == nil {
			testFrame := gocv.NewMat()
			defer testFrame.Close()
			if ok := webcam.Read(&testFrame); ok {
				return webcam, nil
			}
			webcam.Close()
		}
	}

	// Try different backends in order of preference
	backends := []string{
		"0",          // Default V4L2
		"v4l2://",    // Explicit V4L2
		"cvcap://0",  // CV VideoCapture
		"libv4l://0", // libv4l wrapper
	}

	for _, backend := range backends {
		webcam, err := gocv.OpenVideoCapture(backend)
		if err == nil {
			// Test if we can actually read a frame
			testFrame := gocv.NewMat()
			defer testFrame.Close()
			if ok := webcam.Read(&testFrame); ok && !testFrame.Empty() {
				return webcam, nil
			}
			webcam.Close()
		}
	}

	// Try with default one more time
	webcam, err := gocv.OpenVideoCapture(0)
	if err == nil {
		return webcam, nil
	}

	return nil, fmt.Errorf("failed to open any video capture device: %v", err)
}

func runWithMediaPipe() {
	webcam, err := openVideoCapture()
	if err != nil {
		fmt.Printf("Error opening camera: %v\n", err)
		fmt.Println("Attempting fallback to file input or test mode...")
		runTestMode()
		return
	}
	defer webcam.Close()

	window := gocv.NewWindow("Driver Drowsiness Detection - MediaPipe")
	defer window.Close()

	frame := gocv.NewMat()
	defer frame.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	fmt.Println("Driver drowsiness monitor started — press ESC to exit")
	fmt.Println("Using MediaPipe for accurate eye tracking")
	fmt.Println("Green = Face | Yellow = Eyes | Red = Mouth | Red Text = ALERT")

	for {
		if ok := webcam.Read(&frame); !ok {
			continue
		}
		if frame.Empty() {
			continue
		}

		// Send frame to MediaPipe server
		result := detectWithMediaPipe(frame, client)

		// Debug: Print detection result
		if !result.FaceDetected {
			fmt.Println("Face NOT detected in frame")
		} else {
			fmt.Printf("Face detected! EAR: %.2f, FaceBox: %v\n", result.EAR, result.FaceBox)
		}

		if result.FaceDetected {
			// Draw face rectangle (green)
			faceBox := result.FaceBox
			if len(faceBox) == 4 {
				faceRect := image.Rect(faceBox[0], faceBox[1], faceBox[2], faceBox[3])
				gocv.Rectangle(&frame, faceRect, color.RGBA{0, 255, 0, 0}, 2)
			}

			// Draw left eye rectangle (yellow)
			leftEyeBox := result.LeftEyeBox
			if len(leftEyeBox) == 4 {
				leftEyeRect := image.Rect(leftEyeBox[0], leftEyeBox[1], leftEyeBox[2], leftEyeBox[3])
				gocv.Rectangle(&frame, leftEyeRect, color.RGBA{255, 255, 0, 0}, 2)
			}

			// Draw right eye rectangle (yellow)
			rightEyeBox := result.RightEyeBox
			if len(rightEyeBox) == 4 {
				rightEyeRect := image.Rect(rightEyeBox[0], rightEyeBox[1], rightEyeBox[2], rightEyeBox[3])
				gocv.Rectangle(&frame, rightEyeRect, color.RGBA{255, 255, 0, 0}, 2)
			}

			// Draw mouth rectangle (red)
			mouthBox := result.MouthBox
			if len(mouthBox) == 4 {
				mouthRect := image.Rect(mouthBox[0], mouthBox[1], mouthBox[2], mouthBox[3])
				gocv.Rectangle(&frame, mouthRect, color.RGBA{255, 0, 0, 0}, 2)
			}

			// Draw eye landmarks
			for _, p := range result.LeftEye {
				gocv.Circle(&frame, image.Point{X: p[0], Y: p[1]}, 2, color.RGBA{255, 255, 0, 0}, -1)
			}
			for _, p := range result.RightEye {
				gocv.Circle(&frame, image.Point{X: p[0], Y: p[1]}, 2, color.RGBA{255, 255, 0, 0}, -1)
			}

			// Display EAR value
			earText := fmt.Sprintf("EAR: %.2f", result.EAR)
			gocv.PutText(&frame, earText, image.Pt(10, 30),
				gocv.FontHersheySimplex, 0.7, color.RGBA{255, 255, 255, 0}, 1)

			// Check for drowsiness
			if result.EAR < EARThreshold {
				closedFrames++
			} else {
				closedFrames = 0
			}

			if closedFrames > MaxFrames {
				// Print alert to terminal
				fmt.Println("⚠️  ALERT: DROWSINESS DETECTED! Driver may be falling asleep!  ⚠️")

				// Display alert on screen
				gocv.PutText(&frame, "DROWSINESS ALERT!", image.Pt(50, 50),
					gocv.FontHersheySimplex, 1.2, color.RGBA{255, 0, 0, 0}, 3)
			}
		}

		window.IMShow(frame)
		if window.WaitKey(1) == 27 {
			break
		}
	}
}

func detectWithMediaPipe(frame gocv.Mat, client *http.Client) *MediaPipeResponse {
	// Convert frame to JPEG
	buf, err := gocv.IMEncode(".jpg", frame)
	if err != nil {
		fmt.Println("Error encoding frame to JPEG:", err)
		return &MediaPipeResponse{FaceDetected: false}
	}
	defer buf.Close()

	// Encode to base64
	imgBytes := buf.GetBytes()
	b64Bytes := make([]byte, base64.StdEncoding.EncodedLen(len(imgBytes)))
	base64.StdEncoding.Encode(b64Bytes, imgBytes)

	// Send to server
	reqBody, _ := json.Marshal(map[string]string{"image": string(b64Bytes)})
	resp, err := client.Post(MediaPipeURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		fmt.Println("Error posting to MediaPipe server:", err)
		return &MediaPipeResponse{FaceDetected: false}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("MediaPipe server returned status: %d\n", resp.StatusCode)
		return &MediaPipeResponse{FaceDetected: false}
	}

	body, _ := io.ReadAll(resp.Body)
	var result MediaPipeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("Error parsing MediaPipe response:", err)
		return &MediaPipeResponse{FaceDetected: false}
	}

	return &result
}

func detectFaces(img gocv.Mat, net *gocv.Net) []image.Rectangle {
	blob := gocv.BlobFromImage(
		img, 1.0, image.Pt(300, 300),
		gocv.NewScalar(104, 177, 123, 0),
		false, false,
	)
	defer blob.Close()

	net.SetInput(blob, "")
	out := net.Forward("")
	defer out.Close()

	data, _ := out.DataPtrFloat32()
	var faces []image.Rectangle

	for i := 0; i < len(data); i += 7 {
		confidence := data[i+2]
		if confidence > 0.6 {
			x1 := int(data[i+3] * float32(img.Cols()))
			y1 := int(data[i+4] * float32(img.Rows()))
			x2 := int(data[i+5] * float32(img.Cols()))
			y2 := int(data[i+6] * float32(img.Rows()))
			faces = append(faces, image.Rect(x1, y1, x2, y2))
		}
	}
	return faces
}

// runTestMode displays a test pattern when no camera is available
func runTestMode() {
	fmt.Println("Running in test mode - displaying generated frames")

	window := gocv.NewWindow("Driver Drowsiness Detection - Test Mode")
	defer window.Close()

	// Create a test frame with text
	frame := gocv.NewMatWithSize(480, 640, gocv.MatTypeCV8UC3)
	defer frame.Close()

	// Fill with dark gray background
	gocv.Rectangle(&frame, image.Rect(0, 0, 640, 480), color.RGBA{50, 50, 50, 0}, -1)

	// Add test text
	gocv.PutText(&frame, "Camera Not Available", image.Pt(180, 180),
		gocv.FontHersheySimplex, 0.8, color.RGBA{255, 100, 100, 0}, 2)
	gocv.PutText(&frame, "Please connect a camera", image.Pt(200, 230),
		gocv.FontHersheySimplex, 0.6, color.RGBA{200, 200, 200, 0}, 1)
	gocv.PutText(&frame, "Or check camera permissions", image.Pt(190, 270),
		gocv.FontHersheySimplex, 0.6, color.RGBA{200, 200, 200, 0}, 1)

	// Draw sample boxes
	gocv.Rectangle(&frame, image.Rect(200, 300, 300, 380), color.RGBA{0, 255, 0, 0}, 2)   // Face
	gocv.Rectangle(&frame, image.Rect(210, 310, 250, 340), color.RGBA{255, 255, 0, 0}, 2) // Left eye
	gocv.Rectangle(&frame, image.Rect(260, 310, 290, 340), color.RGBA{255, 255, 0, 0}, 2) // Right eye
	gocv.Rectangle(&frame, image.Rect(220, 350, 280, 375), color.RGBA{255, 0, 0, 0}, 2)   // Mouth

	gocv.PutText(&frame, "Test: Boxes should look like this", image.Pt(150, 420),
		gocv.FontHersheySimplex, 0.5, color.RGBA{150, 150, 150, 0}, 1)

	// Try to connect to MediaPipe for test
	if checkMediaPipeServer() {
		fmt.Println("MediaPipe server is running - ready for real detection")
		gocv.PutText(&frame, "MediaPipe: Connected", image.Pt(10, 460),
			gocv.FontHersheySimplex, 0.5, color.RGBA{0, 255, 0, 0}, 1)
	} else {
		fmt.Println("MediaPipe server is NOT running")
		gocv.PutText(&frame, "MediaPipe: Not Connected", image.Pt(10, 460),
			gocv.FontHersheySimplex, 0.5, color.RGBA{255, 0, 0, 0}, 1)
	}

	fmt.Println("Test mode window opened - press ESC to exit")

	for {
		window.IMShow(frame)
		if window.WaitKey(1000) == 27 {
			break
		}
	}
}
