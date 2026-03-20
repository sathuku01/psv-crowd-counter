package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"net/http"
	"os"
	"time"

	"gocv.io/x/gocv"
)

const (
	EARThreshold    = 0.30 // Eye Aspect Ratio threshold for drowsiness (higher = more sensitive)
	MARThreshold    = 0.35 // Mouth Aspect Ratio threshold for yawning (lower = more sensitive)
	PitchThreshold  = 12.0 // Head pitch threshold (looking down = drowsiness) - very low for more sensitivity
	YawThreshold    = 20.0 // Head yaw threshold (looking sideways) - very low for more sensitivity
	RollThreshold   = 15.0 // Head roll threshold (tilting head) - very low for more sensitivity
	MaxFrames       = 8    // Frames with closed eyes to trigger alert (very few for fast detection)
	MaxYawnFrames   = 6    // Frames with yawning to trigger alert (very few for fast detection)
	MaxPoseFrames   = 8    // Frames with abnormal pose to trigger alert (very few for fast detection)
	MediaPipeURL    = "http://localhost:5000/detect"
	smoothingFactor = 0.5 // EMA smoothing factor (higher for faster response)

	// Sliding window settings
	WindowSize           = 30 // 1 second at 30 FPS (minimal for fast detection)
	BlinkFramesThreshold = 5  // Frames to consider a "long" blink (very few for more sensitivity)
)

var (
	closedFrames  int
	yawnFrames    int
	poseFrames    int
	smoothedEAR   float64 = 0.25
	smoothedMAR   float64 = 0.25
	smoothedPitch float64 = 0.0
	smoothedYaw   float64 = 0.0
	smoothedRoll  float64 = 0.0

	// Sliding window buffers for feature computation
	earHistory   []float64
	pitchHistory []float64
	yawHistory   []float64
	rollHistory  []float64
	marHistory   []float64
	blinkHistory []bool // true if eye was closed that frame

	// Computed features
	drowsinessScore    float64 = 0.0
	blinkRate          float64 = 0.0
	slowBlinkRatio     float64 = 0.0
	eyeClosureDuration int     = 0
	nodCount           int     = 0
	yawnDuration       int     = 0

	// State tracking
	wasEyeClosed    bool = false
	blinkStartFrame int  = 0
	nodStartFrame   int  = 0
	isNodding       bool = false
)

// Statistics helper functions
func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func calculateStdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := calculateMean(values)
	sumSq := 0.0
	for _, v := range values {
		diff := v - mean
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(len(values)-1))
}

func calculateMin(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	minVal := values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
	}
	return minVal
}

func calculateMax(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	maxVal := values[0]
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

func countBlinks(blinkHistory []bool) (totalBlinks, slowBlinks int) {
	if len(blinkHistory) < 2 {
		return 0, 0
	}
	inBlink := false
	for i, closed := range blinkHistory {
		if closed && !inBlink {
			// Blink started
			inBlink = true
			blinkStart := i
			// Find blink end
			for j := i + 1; j < len(blinkHistory); j++ {
				if !blinkHistory[j] {
					// Blink ended
					blinkDuration := j - blinkStart
					totalBlinks++
					if blinkDuration >= BlinkFramesThreshold {
						slowBlinks++
					}
					inBlink = false
					break
				}
			}
		}
	}
	return totalBlinks, slowBlinks
}

func countHeadNods(pitchHistory []float64, threshold float64) int {
	if len(pitchHistory) < 10 {
		return 0
	}
	nods := 0
	inNod := false
	for i, pitch := range pitchHistory {
		if pitch > threshold && !inNod {
			inNod = true
			nodStart := i
			// Find nod end (when pitch goes back below threshold)
			for j := i + 1; j < len(pitchHistory); j++ {
				if pitchHistory[j] <= threshold {
					// Nod ended - count only if duration is reasonable (not just noise)
					if j-nodStart >= 5 && j-nodStart <= 30 {
						nods++
					}
					inNod = false
					break
				}
			}
		}
	}
	return nods
}

func calculateDrowsinessScore(
	earMean, earMin float64,
	blinkRate, slowBlinkRatio float64,
	nodCount, yawnDuration, eyeClosureDuration int,
	pitchMax, yawMax, rollMax float64,
) float64 {
	// Weighted drowsiness score
	score := 0.0

	// Eye closure contributes most (40%)
	if earMean < EARThreshold {
		score += 0.4 * (1.0 - earMean/EARThreshold)
	}
	if earMin < EARThreshold*0.5 {
		score += 0.2
	}

	// Eye closure duration (20%)
	if eyeClosureDuration > 15 {
		score += 0.2 * math.Min(1.0, float64(eyeClosureDuration)/40.0)
	}

	// Slow blinks indicate drowsiness (15%)
	score += 0.15 * slowBlinkRatio

	// Head nodding is strong indicator (15%)
	if nodCount >= 2 {
		score += 0.15 * math.Min(1.0, float64(nodCount)/4.0)
	}

	// Yawning (10%)
	if yawnDuration > 10 {
		score += 0.1 * math.Min(1.0, float64(yawnDuration)/20.0)
	}

	return math.Min(1.0, score)
}

// MediaPipeResponse represents the response from MediaPipe server
type MediaPipeResponse struct {
	FaceDetected bool    `json:"face_detected"`
	EAR          float64 `json:"ear"`
	LeftEAR      float64 `json:"left_ear"`
	RightEAR     float64 `json:"right_ear"`
	MAR          float64 `json:"mar"`
	Pitch        float64 `json:"pitch"` // Head nod (positive = looking down)
	Yaw          float64 `json:"yaw"`   // Head turn (positive = looking right)
	Roll         float64 `json:"roll"`  // Head tilt (positive = tilting right)
	FaceBox      []int   `json:"face_box"`
	LeftEyeBox   []int   `json:"left_eye_box"`
	RightEyeBox  []int   `json:"right_eye_box"`
	MouthBox     []int   `json:"mouth_box"`
	LeftEye      [][]int `json:"left_eye"`
	RightEye     [][]int `json:"right_eye"`
	Nose         [][]int `json:"nose"`
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

	fmt.Println("Running with face detection only.\nPlease start mediapipe server")

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
			// Video ended - close window and exit
			fmt.Println("Video ended, closing window...")
			break
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
			fmt.Printf("Face detected! EAR: %.2f, MAR: %.2f, Pitch: %.1f, Yaw: %.1f, Roll: %.1f, FaceBox: %v, Nose: %v\n",
				result.EAR, result.MAR, result.Pitch, result.Yaw, result.Roll, result.FaceBox, result.Nose)
		}

		if result.FaceDetected {
			// ===== SLIDING WINDOW FEATURE COMPUTATION =====
			// Update sliding window buffers
			earHistory = append(earHistory, result.EAR)
			if len(earHistory) > WindowSize {
				earHistory = earHistory[len(earHistory)-WindowSize:]
			}

			pitchHistory = append(pitchHistory, result.Pitch)
			if len(pitchHistory) > WindowSize {
				pitchHistory = pitchHistory[len(pitchHistory)-WindowSize:]
			}

			yawHistory = append(yawHistory, result.Yaw)
			if len(yawHistory) > WindowSize {
				yawHistory = yawHistory[len(yawHistory)-WindowSize:]
			}

			rollHistory = append(rollHistory, result.Roll)
			if len(rollHistory) > WindowSize {
				rollHistory = rollHistory[len(rollHistory)-WindowSize:]
			}

			marHistory = append(marHistory, result.MAR)
			if len(marHistory) > WindowSize {
				marHistory = marHistory[len(marHistory)-WindowSize:]
			}

			// Track eye state for blink detection
			eyeClosedNow := result.EAR < EARThreshold
			blinkHistory = append(blinkHistory, eyeClosedNow)
			if len(blinkHistory) > WindowSize {
				blinkHistory = blinkHistory[len(blinkHistory)-WindowSize:]
			}

			// Calculate features when we have enough data
			if len(earHistory) >= 10 {
				earMean := calculateMean(earHistory)
				earMin := calculateMin(earHistory)

				totalBlinks, slowBlinks := countBlinks(blinkHistory)
				if len(blinkHistory) > 0 {
					blinkRate = float64(totalBlinks) * 30.0 / float64(len(blinkHistory)) // blinks per minute
				}
				if totalBlinks > 0 {
					slowBlinkRatio = float64(slowBlinks) / float64(totalBlinks)
				}

				nodCount = countHeadNods(pitchHistory, PitchThreshold)

				// Calculate drowsiness score
				drowsinessScore = calculateDrowsinessScore(
					earMean, earMin,
					blinkRate, slowBlinkRatio,
					nodCount, yawnDuration, eyeClosureDuration,
					calculateMax(pitchHistory), calculateMax(yawHistory), calculateMax(rollHistory),
				)
			}

			// Track blink durations
			if eyeClosedNow {
				if !wasEyeClosed {
					// Blink started
					blinkStartFrame = len(earHistory)
				}
				eyeClosureDuration++
			} else {
				if wasEyeClosed {
					// Blink ended
					if eyeClosureDuration > BlinkFramesThreshold {
						// This was a long blink
					}
				}
				eyeClosureDuration = 0
			}
			wasEyeClosed = eyeClosedNow

			// Track yawning duration
			if result.MAR > MARThreshold {
				yawnDuration++
			} else {
				if yawnDuration > 15 {
					// Significant yawn detected
				}
				yawnDuration = 0
			}

			// ===== DRAWING =====
			// Draw face rectangle (green)
			faceBox := result.FaceBox
			if len(faceBox) == 4 {
				faceRect := image.Rect(faceBox[0], faceBox[1], faceBox[2], faceBox[3])
				gocv.Rectangle(&frame, faceRect, color.RGBA{0, 255, 0, 0}, 2)
			}

			// Draw mouth rectangle (red)
			mouthBox := result.MouthBox
			if len(mouthBox) == 4 {
				mouthRect := image.Rect(mouthBox[0], mouthBox[1], mouthBox[2], mouthBox[3])
				gocv.Rectangle(&frame, mouthRect, color.RGBA{255, 0, 0, 0}, 2)
			}

			// Draw eye landmarks only (no bounding boxes)
			for _, p := range result.LeftEye {
				gocv.Circle(&frame, image.Point{X: p[0], Y: p[1]}, 2, color.RGBA{255, 255, 0, 0}, -1)
			}
			for _, p := range result.RightEye {
				gocv.Circle(&frame, image.Point{X: p[0], Y: p[1]}, 2, color.RGBA{255, 255, 0, 0}, -1)
			}

			// Draw only nose tip (first point, larger purple dot)
			if len(result.Nose) > 0 {
				gocv.Circle(&frame, image.Point{X: result.Nose[0][0], Y: result.Nose[0][1]}, 5, color.RGBA{255, 0, 255, 0}, -1)
			}

			// Display comprehensive features
			var statusText string
			var statusColor color.RGBA

			if drowsinessScore > 0.7 {
				statusText = "ALERT!"
				statusColor = color.RGBA{255, 0, 0, 0}
			} else if drowsinessScore > 0.4 {
				statusText = "DROWSY"
				statusColor = color.RGBA{255, 165, 0, 0}
			} else {
				statusText = "Normal"
				statusColor = color.RGBA{0, 255, 0, 0}
			}

			// Main status line with score
			mainText := fmt.Sprintf("Status: %s | Drowsiness: %.0f%%", statusText, drowsinessScore*100)
			gocv.PutText(&frame, mainText, image.Pt(10, 30),
				gocv.FontHersheySimplex, 0.6, statusColor, 2)

			// EAR and MAR
			earMarText := fmt.Sprintf("EAR: %.2f | MAR: %.2f", result.EAR, result.MAR)
			gocv.PutText(&frame, earMarText, image.Pt(10, 60),
				gocv.FontHersheySimplex, 0.5, color.RGBA{255, 255, 255, 0}, 1)

			// Head pose
			poseText := fmt.Sprintf("Pitch: %.1f° | Yaw: %.1f° | Roll: %.1f°", smoothedPitch, smoothedYaw, smoothedRoll)
			gocv.PutText(&frame, poseText, image.Pt(10, 85),
				gocv.FontHersheySimplex, 0.5, color.RGBA{200, 200, 200, 0}, 1)

			// Blink and nod features
			// Compute nod count for display if not computed yet
			if nodCount == 0 && len(pitchHistory) >= 10 {
				nodCount = countHeadNods(pitchHistory, PitchThreshold)
			}
			featureText := fmt.Sprintf("Blink Rate: %.1f/min | Slow Blink: %.0f%% | Nods: %d",
				blinkRate, slowBlinkRatio*100, nodCount)
			gocv.PutText(&frame, featureText, image.Pt(10, 110),
				gocv.FontHersheySimplex, 0.4, color.RGBA{150, 150, 150, 0}, 1)

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
				gocv.PutText(&frame, "DROWSINESS ALERT!", image.Pt(0, 250),
					gocv.FontHersheySimplex, 1.2, color.RGBA{255, 0, 0, 0}, 3)
			}

			// Apply EMA smoothing for more stable detection
			if result.EAR > 0 {
				smoothedEAR = smoothedEAR*(1-smoothingFactor) + result.EAR*smoothingFactor
			}
			if result.MAR > 0 {
				smoothedMAR = smoothedMAR*(1-smoothingFactor) + result.MAR*smoothingFactor
			}

			// Check for yawning using smoothed MAR
			if smoothedMAR > MARThreshold && result.MAR > MARThreshold*0.8 {
				yawnFrames++
			} else {
				yawnFrames = 0
			}

			if yawnFrames > MaxYawnFrames {
				fmt.Println("⚠️  ALERT: YAWING DETECTED! Driver may be fatigued!  ⚠️")
				gocv.PutText(&frame, "YAWNING ALERT!", image.Pt(0, 200),
					gocv.FontHersheySimplex, 1.0, color.RGBA{255, 165, 0, 0}, 3)
			}

			// Apply EMA smoothing to head pose
			if result.Pitch != 0 {
				smoothedPitch = smoothedPitch*(1-smoothingFactor) + result.Pitch*smoothingFactor
			}
			if result.Yaw != 0 {
				smoothedYaw = smoothedYaw*(1-smoothingFactor) + result.Yaw*smoothingFactor
			}
			if result.Roll != 0 {
				smoothedRoll = smoothedRoll*(1-smoothingFactor) + result.Roll*smoothingFactor
			}

			// Check for abnormal head pose (drowsiness indicator)
			// Positive pitch = looking down (chin to chest) - main drowsiness indicator
			// Large yaw = looking sideways
			// Large roll = tilting head
			isDrowsyPose := smoothedPitch > PitchThreshold ||
				smoothedYaw > YawThreshold ||
				smoothedYaw < -YawThreshold ||
				smoothedRoll > RollThreshold ||
				smoothedRoll < -RollThreshold

			if isDrowsyPose {
				poseFrames++
			} else {
				poseFrames = 0
			}

			if poseFrames > MaxPoseFrames {
				// Determine which pose issue and display single colored alert
				var poseAlert string
				var poseColor color.RGBA

				if smoothedPitch > PitchThreshold {
					poseAlert = "HEAD DOWN"
					poseColor = color.RGBA{0, 100, 255, 0} // Blue
				} else if smoothedYaw > YawThreshold || smoothedYaw < -YawThreshold {
					poseAlert = "HEAD TURN"
					poseColor = color.RGBA{255, 0, 255, 0} // Magenta
				} else if smoothedRoll > RollThreshold || smoothedRoll < -RollThreshold {
					poseAlert = "HEAD TILT"
					poseColor = color.RGBA{0, 255, 255, 0} // Cyan
				}

				if poseAlert != "" {
					fmt.Println("⚠️  ALERT: " + poseAlert + "! Driver may be drowsy!  ⚠️")
					gocv.PutText(&frame, poseAlert+" ALERT!", image.Pt(0, 150),
						gocv.FontHersheySimplex, 0.9, poseColor, 3)
				}

				// ===== COMBINED DROWSINESS ALERT =====
				// Show combined alert if any drowsiness indicator is triggered
				if closedFrames > MaxFrames || yawnFrames > MaxYawnFrames || poseFrames > MaxPoseFrames {
					var alertMsg string
					alertMsg = "DROWSINESS: "
					hasMultiple := false

					if closedFrames > MaxFrames {
						alertMsg += "EyesClosed"
						hasMultiple = true
					}
					if yawnFrames > MaxYawnFrames {
						if hasMultiple {
							alertMsg += "+"
						}
						alertMsg += "Yawning"
						hasMultiple = true
					}
					if poseFrames > MaxPoseFrames {
						if hasMultiple {
							alertMsg += "+"
						}
						if smoothedPitch > PitchThreshold {
							alertMsg += "HeadDown"
						}
						if smoothedYaw > YawThreshold || smoothedYaw < -YawThreshold {
							if smoothedPitch > PitchThreshold {
								alertMsg += "+"
							}
							alertMsg += "HeadTurn"
						}
						if smoothedRoll > RollThreshold || smoothedRoll < -RollThreshold {
							alertMsg += "+HeadTilt"
						}
					}

					// Terminal alert
					fmt.Println(">>> ALERT: " + alertMsg + " <<<")

					// // Show what triggered
					yy := 90
					if closedFrames > MaxFrames {
						gocv.PutText(&frame, "- Eyes closed", image.Pt(450, 60), gocv.FontHersheySimplex, 0.7, color.RGBA{255, 255, 0, 0}, 2)
						yy += 30
					}
					if yawnFrames > MaxYawnFrames {
						gocv.PutText(&frame, "- Yawning", image.Pt(450, 80), gocv.FontHersheySimplex, 0.7, color.RGBA{255, 165, 0, 0}, 2)
						yy += 30
					}
					if poseFrames > MaxPoseFrames {
						gocv.PutText(&frame, "- Head pose alert", image.Pt(450, yy), gocv.FontHersheySimplex, 0.7, color.RGBA{0, 100, 255, 0}, 2)
					}
				}

			}
		}

		window.IMShow(frame)
		key := window.WaitKey(1)
		// ESC key to exit
		if key == 27 {
			fmt.Println("ESC pressed, exiting...")
			break
		}
	}
}

func detectWithMediaPipe(frame gocv.Mat, client *http.Client) *MediaPipeResponse {
	// Debug: Print frame size
	fmt.Printf("[DEBUG] Frame size: %dx%d, channels: %d\n", frame.Cols(), frame.Rows(), frame.Channels())

	// Convert frame to JPEG
	buf, err := gocv.IMEncode(".jpg", frame)
	if err != nil {
		fmt.Println("Error encoding frame to JPEG:", err)
		return &MediaPipeResponse{FaceDetected: false}
	}
	defer buf.Close()

	// Encode to base64
	imgBytes := buf.GetBytes()
	fmt.Printf("[DEBUG] JPEG size: %d bytes\n", len(imgBytes))
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
