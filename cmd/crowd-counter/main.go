package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"sort"

	"gocv.io/x/gocv"
)

// Detection holds a single bounding box and its score
type Detection struct {
	box   image.Rectangle
	score float32
	class int
}

func main() {
	// Video path
	videoPath := "/home/sathuku/psv-crowd-counter/internal/camera/mock/sample.mp4"
	// YOLOv8 ONNX path
	modelPath := "/home/sathuku/psv-crowd-counter/yolov8n.onnx"

	// Open video file
	video, err := gocv.VideoCaptureFile(videoPath)
	if err != nil {
		log.Fatalf("Error opening video: %v\n", err)
	}
	defer video.Close()

	// Get video properties
	width := int(video.Get(gocv.VideoCaptureFrameWidth))
	height := int(video.Get(gocv.VideoCaptureFrameHeight))
	fmt.Printf("Video dimensions: %dx%d\n", width, height)

	// Create window
	window := gocv.NewWindow("YOLOv8 Person Detection")
	defer window.Close()

	// Load YOLOv8 ONNX model
	net := gocv.ReadNet(modelPath, "")
	if net.Empty() {
		log.Fatal("Failed to load YOLOv8 ONNX model")
	}
	defer net.Close()

	// Set preferable backend and target
	net.SetPreferableBackend(gocv.NetBackendDefault)
	net.SetPreferableTarget(gocv.NetTargetCPU)

	// Mat to hold frames
	img := gocv.NewMat()
	defer img.Close()

	// Colors for different classes
	green := color.RGBA{0, 255, 0, 0}
	white := color.RGBA{255, 255, 255, 0}
	black := color.RGBA{0, 0, 0, 0}

	frameCount := 0
	inputSize := 640 // YOLOv8 input size
	confThreshold := float32(0.25)  // Confidence threshold
	nmsThreshold := float32(0.45)    // NMS threshold

	for {
		if ok := video.Read(&img); !ok {
			fmt.Println("End of video")
			break
		}
		if img.Empty() {
			continue
		}

		frameCount++
		
		// Prepare input blob
		blob := gocv.BlobFromImage(
			img,
			1.0/255.0,
			image.Pt(inputSize, inputSize),
			gocv.NewScalar(0, 0, 0, 0),
			true,  // swapRB (BGR to RGB)
			false, // crop
		)
		
		net.SetInput(blob, "")
		output := net.Forward("")
		blob.Close()

		// Get output data
		data, err := output.DataPtrFloat32()
		if err != nil {
			log.Printf("Error getting data pointer: %v", err)
			output.Close()
			continue
		}

		// Get output shape
		size := output.Size()
		
		// Debug: print output shape for first frame
		if frameCount == 1 {
			fmt.Printf("Output shape: %v\n", size)
			fmt.Printf("Total elements: %d\n", len(data))
		}

		detections := []Detection{}

		// YOLOv8 output format handling
		if len(size) == 3 {
			// Standard YOLOv8 output: [1, num_classes+4, num_predictions]
			// where num_classes = 80 for COCO, +4 for bbox coordinates
			numPredictions := size[2] // Usually 8400
			numChannels := size[1]     // Usually 84 (4 bbox + 80 classes)
			
			if frameCount == 1 {
				fmt.Printf("Parsing YOLOv8 output: %d predictions, %d channels\n", 
					numPredictions, numChannels)
				fmt.Printf("Sample data - first few values: %v\n", data[:min(20, len(data))])
			}
			
			// Calculate scaling factors
			scaleX := float32(img.Cols()) / float32(inputSize)
			scaleY := float32(img.Rows()) / float32(inputSize)
			
			// For each prediction
			for i := 0; i < numPredictions; i++ {
				// Find the best class score
				bestClass := -1
				bestScore := float32(0)
				
				// Class scores start at index 4 (after bbox coordinates)
				for j := 4; j < numChannels; j++ {
					// Access: data[channel * numPredictions + prediction]
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
				
				// Check if it's a person (class 0) with sufficient confidence
				if bestClass == 0 && bestScore > confThreshold {
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
					left = max(0, min(left, img.Cols()))
					top = max(0, min(top, img.Rows()))
					right = max(0, min(right, img.Cols()))
					bottom = max(0, min(bottom, img.Rows()))
					
					// Filter out invalid boxes
					if right > left && bottom > top {
						detections = append(detections, Detection{
							box:   image.Rect(left, top, right, bottom),
							score: bestScore,
							class: bestClass,
						})
						
						// Debug first few detections
						if frameCount == 1 && len(detections) <= 5 {
							fmt.Printf("  Detection %d: class=%d, score=%.2f, box=(%d,%d,%d,%d)\n", 
								len(detections), bestClass, bestScore, left, top, right, bottom)
						}
					}
				}
			}
		} else {
			// Alternative format: [1, num_predictions, num_channels]
			numPredictions := size[1]
			numChannels := size[2]
			
			if frameCount == 1 {
				fmt.Printf("Alternative format: %d predictions, %d channels\n", 
					numPredictions, numChannels)
			}
			
			scaleX := float32(img.Cols()) / float32(inputSize)
			scaleY := float32(img.Rows()) / float32(inputSize)
			
			for i := 0; i < numPredictions; i++ {
				baseIdx := i * numChannels
				if baseIdx+numChannels > len(data) {
					continue
				}
				
				// Get bbox coordinates
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
				
				if bestClass == 0 && bestScore > confThreshold {
					left := int((cx - w/2) * scaleX)
					top := int((cy - h/2) * scaleY)
					right := int((cx + w/2) * scaleX)
					bottom := int((cy + h/2) * scaleY)
					
					left = max(0, min(left, img.Cols()))
					top = max(0, min(top, img.Rows()))
					right = max(0, min(right, img.Cols()))
					bottom = max(0, min(bottom, img.Rows()))
					
					if right > left && bottom > top {
						detections = append(detections, Detection{
							box:   image.Rect(left, top, right, bottom),
							score: bestScore,
							class: bestClass,
						})
					}
				}
			}
		}
		
		output.Close()

		// Apply Non-Maximum Suppression
		finalDetections := nms(detections, nmsThreshold)

		// Draw detections
		for _, d := range finalDetections {
			if d.class != 0 {
				continue // Skip non-person detections
			}
			
			// Draw rectangle
			gocv.Rectangle(&img, d.box, green, 2)
			
			// Draw label with confidence
			label := fmt.Sprintf("Person %.2f", d.score)
			
			// Get text size for background
			textSize := gocv.GetTextSize(label, gocv.FontHersheySimplex, 0.5, 1)
			
			// Draw text background
			textBg := image.Rect(
				d.box.Min.X,
				d.box.Min.Y-25,
				d.box.Min.X+textSize.X+10,
				d.box.Min.Y,
			)
			
			// Adjust if text would go above frame
			if textBg.Min.Y < 0 {
				textBg = image.Rect(
					d.box.Min.X,
					d.box.Min.Y,
					d.box.Min.X+textSize.X+10,
					d.box.Min.Y+25,
				)
			}
			
			gocv.Rectangle(&img, textBg, black, -1)
			
			// Draw text
			textPt := image.Pt(d.box.Min.X+5, d.box.Min.Y-5)
			if textBg.Min.Y == d.box.Min.Y {
				textPt = image.Pt(d.box.Min.X+5, d.box.Min.Y+17)
			}
			
			gocv.PutText(
				&img,
				label,
				textPt,
				gocv.FontHersheySimplex,
				0.5,
				white,
				1,
			)
		}

		// Display count
		countLabel := fmt.Sprintf("People: %d", len(finalDetections))
		gocv.PutText(
			&img,
			countLabel,
			image.Pt(10, 30),
			gocv.FontHersheySimplex,
			1.0,
			green,
			2,
		)

		// Print stats periodically
		if frameCount%30 == 0 {
			fmt.Printf("Frame %d: %d raw detections, %d people after NMS\n", 
				frameCount, len(detections), len(finalDetections))
		}

		window.IMShow(img)
		if window.WaitKey(1) >= 0 {
			break
		}
	}

	fmt.Printf("\nProcessing complete. Total frames: %d\n", frameCount)
}

// Non-Maximum Suppression
func nms(detections []Detection, iouThreshold float32) []Detection {
	if len(detections) == 0 {
		return detections
	}

	// Sort by score descending
	sort.Slice(detections, func(i, j int) bool {
		return detections[i].score > detections[j].score
	})

	result := []Detection{}
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
			
			if iou(detections[i].box, detections[j].box) > iouThreshold {
				used[j] = true
			}
		}
	}
	
	return result
}

// Calculate Intersection over Union
func iou(a, b image.Rectangle) float32 {
	// Calculate intersection
	x1 := float32(max(a.Min.X, b.Min.X))
	y1 := float32(max(a.Min.Y, b.Min.Y))
	x2 := float32(min(a.Max.X, b.Max.X))
	y2 := float32(min(a.Max.Y, b.Max.Y))

	interWidth := float32(max(0, int(x2-x1)))
	interHeight := float32(max(0, int(y2-y1)))
	intersection := interWidth * interHeight
	
	if intersection == 0 {
		return 0
	}
	
	// Calculate union
	areaA := float32((a.Max.X - a.Min.X) * (a.Max.Y - a.Min.Y))
	areaB := float32((b.Max.X - b.Min.X) * (b.Max.Y - b.Min.Y))
	union := areaA + areaB - intersection
	
	return intersection / union
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}