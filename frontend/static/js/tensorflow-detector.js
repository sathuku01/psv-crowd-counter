/**
 * TensorFlow.js Client-Side Detection
 * This runs COCO-SSD model for high-accuracy person detection
 */

class ClientSideDetector {
    constructor() {
        this.model = null;
        this.isLoaded = false;
        this.isProcessing = false;

        // Temporal smoothing for stable bounding boxes
        this.previousDetections = [];
        this.smoothingFactor = 0.7; // Higher = more stable but slower response
        this.maxHistoryFrames = 5; // Frames to keep for smoothing
    }

    // Load the COCO-SSD model for person detection
    async loadModel(modelPath = null) {
        console.log('Loading COCO-SSD model...');
        
        // Load TensorFlow.js and COCO-SSD from CDN
        await this.loadTFJS();
        
        // Try loading COCO-SSD model (specialized for person detection)
        try {
            // COCO-SSD is optimized for person detection with the 'person' class
            this.model = await cocoSsd.load({
                base: 'lite_mobilenet_v2' // Faster and lighter for browser
            });
            this.isLoaded = true;
            console.log('COCO-SSD model loaded successfully');
            return true;
        } catch (error) {
            console.warn('Failed to load COCO-SSD model:', error.message);
        }
        
        // Fallback to simulation mode
        console.warn('No model available, using simulation mode');
        this.isLoaded = false;
        return false;
    }

    // Load TensorFlow.js library and COCO-SSD from CDN
    async loadTFJS() {
        return new Promise((resolve, reject) => {
            if (window.tf && window.cocoSsd) {
                resolve();
                return;
            }

            let loadedCount = 0;
            const totalScripts = 2;
            const checkLoaded = () => {
                loadedCount++;
                if (loadedCount === totalScripts) {
                    resolve();
                }
            };

            // Load TensorFlow.js
            if (!window.tf) {
                const tfScript = document.createElement('script');
                tfScript.src = 'https://cdn.jsdelivr.net/npm/@tensorflow/tfjs@4.10.0/dist/tf.min.js';
                tfScript.onload = checkLoaded;
                tfScript.onerror = reject;
                document.head.appendChild(tfScript);
            } else {
                loadedCount++;
            }

            // Load COCO-SSD
            if (!window.cocoSsd) {
                const cocoScript = document.createElement('script');
                cocoScript.src = 'https://cdn.jsdelivr.net/npm/@tensorflow-models/coco-ssd@2.2.2/dist/coco-ssd.min.js';
                cocoScript.onload = checkLoaded;
                cocoScript.onerror = reject;
                document.head.appendChild(cocoScript);
            } else {
                loadedCount++;
            }
        });
    }

    // Detect persons in an image
    async detect(imageElement) {
        // If model not loaded, return simulated detection
        if (!this.isLoaded || !this.model) {
            return this.simulateDetection(imageElement);
        }

        if (this.isProcessing) {
            return null;
        }

        this.isProcessing = true;

        try {
            // Run inference with COCO-SSD model
            const predictions = await this.model.detect(imageElement);
            
            // Process predictions to get person detections only
            const result = this.processOutput(predictions, imageElement.width, imageElement.height);
            
            this.isProcessing = false;
            return result;
        } catch (error) {
            console.error('Detection error:', error);
            this.isProcessing = false;
            return this.simulateDetection(imageElement);
        }
    }
    
    // Simulate detection when model is not available
    simulateDetection(imageElement) {
        const width = imageElement.width || 640;
        const height = imageElement.height || 480;
        
        // Generate random detection for demo purposes
        const boxCount = Math.floor(Math.random() * 3);
        const boxes = [];
        
        for (let i = 0; i < boxCount; i++) {
            boxes.push({
                x1: Math.random() * (width - 100),
                y1: Math.random() * (height - 100),
                x2: Math.random() * 100 + 50,
                y2: Math.random() * 100 + 50,
                score: Math.random() * 0.3 + 0.5
            });
        }
        
        return {
            count: boxes.length,
            boxes: boxes,
            detections: boxes,
            simulated: true
        };
    }

    // Process COCO-SSD output to extract person detections
    processOutput(predictions, imageWidth, imageHeight) {
        const boxes = [];
        
        // COCO-SSD returns predictions with bbox, class, and score
        // Class 'person' is typically class 1 in COCO dataset
        const confidenceThreshold = 0.6;
        
        for (const pred of predictions) {
            // Only keep 'person' class with high confidence
            if (pred.class !== 'person' || pred.score < confidenceThreshold) {
                continue;
            }
            
            // Extract bounding box coordinates (normalized 0-1)
            const [x, y, w, h] = pred.bbox;
            
            // Convert to pixel coordinates
            const x1 = x * imageWidth;
            const y1 = y * imageHeight;
            const x2 = (x + w) * imageWidth;
            const y2 = (y + h) * imageHeight;
            
            boxes.push({
                x1: Math.max(0, x1),
                y1: Math.max(0, y1),
                x2: Math.min(imageWidth, x2),
                y2: Math.min(imageHeight, y2),
                score: pred.score
            });
        }

        // Apply NMS to remove overlapping boxes
        const finalBoxes = this.nms(boxes, 0.3);
        
        // Limit to maximum 10 detections
        const filteredBoxes = finalBoxes.slice(0, 10);
        
        // Apply temporal smoothing for stability
        const smoothedBoxes = this.smoothDetections(filteredBoxes);

        return {
            count: smoothedBoxes.length,
            boxes: smoothedBoxes,
            detections: smoothedBoxes
        };
    }

    // Non-maximum suppression
    nms(boxes, iouThreshold) {
        if (boxes.length === 0) return [];

        // Sort by score
        boxes.sort((a, b) => b.score - a.score);

        const keep = [];
        const used = new Array(boxes.length).fill(false);

        for (let i = 0; i < boxes.length; i++) {
            if (used[i]) continue;

            keep.push(boxes[i]);
            
            for (let j = i + 1; j < boxes.length; j++) {
                if (used[j]) continue;
                
                const iou = this.calculateIOU(boxes[i], boxes[j]);
                if (iou > iouThreshold) {
                    used[j] = true;
                }
            }
        }

        return keep;
    }

    // Calculate Intersection over Union
    calculateIOU(box1, box2) {
        const x1 = Math.max(box1.x1, box2.x1);
        const y1 = Math.max(box1.y1, box2.y1);
        const x2 = Math.min(box1.x2, box2.x2);
        const y2 = Math.min(box1.y2, box2.y2);

        if (x2 <= x1 || y2 <= y1) return 0;

        const intersection = (x2 - x1) * (y2 - y1);
        const area1 = (box1.x2 - box1.x1) * (box1.y2 - box1.y1);
        const area2 = (box2.x2 - box2.x1) * (box2.y2 - box2.y1);
        const union = area1 + area2 - intersection;

        return intersection / union;
    }

    // Apply temporal smoothing to detections for stability
    smoothDetections(currentDetections) {
        // Add current detections to history
        this.previousDetections.push(currentDetections);
        if (this.previousDetections.length > this.maxHistoryFrames) {
            this.previousDetections.shift();
        }

        if (this.previousDetections.length < 2) {
            return currentDetections; // Not enough history for smoothing
        }

        // Smooth each detection by averaging with previous frames
        const smoothedDetections = currentDetections.map(currentBox => {
            let smoothedBox = { ...currentBox };

            // Find best matching box from previous frames
            for (let frame of this.previousDetections.slice(0, -1)) {
                const match = this.findBestMatchingBox(currentBox, frame);
                if (match) {
                    // Apply exponential moving average
                    smoothedBox.x1 = smoothedBox.x1 * (1 - this.smoothingFactor) + match.x1 * this.smoothingFactor;
                    smoothedBox.y1 = smoothedBox.y1 * (1 - this.smoothingFactor) + match.y1 * this.smoothingFactor;
                    smoothedBox.x2 = smoothedBox.x2 * (1 - this.smoothingFactor) + match.x2 * this.smoothingFactor;
                    smoothedBox.y2 = smoothedBox.y2 * (1 - this.smoothingFactor) + match.y2 * this.smoothingFactor;
                    smoothedBox.score = Math.max(smoothedBox.score, match.score); // Keep highest confidence
                    break; // Use first good match
                }
            }

            return smoothedBox;
        });

        return smoothedDetections;
    }

    // Find best matching bounding box from previous frame using IoU
    findBestMatchingBox(currentBox, previousDetections) {
        let bestMatch = null;
        let bestIoU = 0;

        for (let prevBox of previousDetections) {
            const iou = this.calculateIOU(currentBox, prevBox);
            if (iou > 0.3 && iou > bestIoU) { // IoU threshold for matching
                bestMatch = prevBox;
                bestIoU = iou;
            }
        }

        return bestMatch;
    }

    // Check if model is loaded
    isReady() {
        return this.isLoaded;
    }
}

// Export for use in other scripts
window.ClientSideDetector = ClientSideDetector;