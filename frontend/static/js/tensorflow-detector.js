/**
 * TensorFlow.js Client-Side Detection
 * This runs YOLOv8 model directly in the browser for real-time person detection
 */

class ClientSideDetector {
    constructor() {
        this.model = null;
        this.isLoaded = false;
        this.isProcessing = false;
    }

    // Load the YOLOv8 model (using a pre-converted TensorFlow.js model)
    async loadModel(modelPath = '/static/models/yolov8n_webmodel/model.json') {
        console.log('Loading TensorFlow.js model...');
        
        // Import TensorFlow.js
        if (!window.tf) {
            // Load TensorFlow.js from CDN
            await this.loadTFJS();
        }
        
        try {
            // Load the model
            this.model = await tf.loadGraphModel(modelPath);
            this.isLoaded = true;
            console.log('Model loaded successfully');
            return true;
        } catch (error) {
            console.error('Failed to load model:', error);
            return false;
        }
    }

    // Load TensorFlow.js library from CDN
    async loadTFJS() {
        return new Promise((resolve, reject) => {
            if (window.tf) {
                resolve();
                return;
            }
            
            const script = document.createElement('script');
            script.src = 'https://cdn.jsdelivr.net/npm/@tensorflow/tfjs@4.10.0/dist/tf.min.js';
            script.onload = resolve;
            script.onerror = reject;
            document.head.appendChild(script);
        });
    }

    // Detect persons in an image
    async detect(imageElement) {
        if (!this.isLoaded || !this.model) {
            console.warn('Model not loaded');
            return null;
        }

        if (this.isProcessing) {
            return null;
        }

        this.isProcessing = true;

        try {
            // Convert image to tensor
            const tensor = tf.browser.fromPixels(imageElement);
            
            // Resize to model input size (640x640 for YOLOv8)
            const resized = tf.image.resizeBilinear(tensor, [640, 640]);
            
            // Normalize to 0-1 range
            const normalized = resized.div(255.0);
            
            // Add batch dimension
            const batched = normalized.expandDims(0);
            
            // Run inference
            const output = this.model.predict(batched);
            
            // Process output to get person detections
            const result = this.processOutput(output, imageElement.width, imageElement.height);
            
            // Clean up tensors
            tensor.dispose();
            resized.dispose();
            normalized.dispose();
            batched.dispose();
            output.forEach(t => t.dispose());

            this.isProcessing = false;
            return result;
        } catch (error) {
            console.error('Detection error:', error);
            this.isProcessing = false;
            return null;
        }
    }

    // Process YOLOv8 output to extract person detections
    processOutput(output, imageWidth, imageHeight) {
        // This is a simplified version - real YOLOv8 output processing is more complex
        // For now, we'll create a placeholder that shows the concept works
        
        const boxes = [];
        const rawOutput = output[0].dataSync();
        
        // YOLOv8 output format: [batch, num_boxes, num_classes + 4]
        // For YOLOv8n with COCO: [1, 8400, 84] (84 = 80 classes + 4 bbox)
        // Person class is index 0
        
        const numBoxes = 8400;
        const numClasses = 80;
        
        for (let i = 0; i < numBoxes; i++) {
            // Get the class scores (starting at index 4)
            let maxScore = 0;
            let maxClass = -1;
            
            for (let c = 0; c < numClasses; c++) {
                const score = rawOutput[i * (numClasses + 4) + 4 + c];
                if (score > maxScore) {
                    maxScore = score;
                    maxClass = c;
                }
            }
            
            // Only care about person (class 0) with sufficient confidence
            if (maxClass === 0 && maxScore > 0.25) {
                // Get bbox coordinates
                const x = rawOutput[i * (numClasses + 4)];
                const y = rawOutput[i * (numClasses + 4) + 1];
                const w = rawOutput[i * (numClasses + 4) + 2];
                const h = rawOutput[i * (numClasses + 4) + 3];
                
                // Convert from center format to corner format
                const x1 = (x - w / 2) * (imageWidth / 640);
                const y1 = (y - h / 2) * (imageHeight / 640);
                const x2 = (x + w / 2) * (imageWidth / 640);
                const y2 = (y + h / 2) * (imageHeight / 640);
                
                boxes.push({
                    x1: Math.max(0, Math.min(imageWidth, x1)),
                    y1: Math.max(0, Math.min(imageHeight, y1)),
                    x2: Math.max(0, Math.min(imageWidth, x2)),
                    y2: Math.max(0, Math.min(imageHeight, y2)),
                    score: maxScore
                });
            }
        }

        // Apply non-maximum suppression
        const finalBoxes = this.nms(boxes, 0.45);
        
        return {
            count: finalBoxes.length,
            boxes: finalBoxes,
            detections: finalBoxes
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

    // Check if model is loaded
    isReady() {
        return this.isLoaded;
    }
}

// Export for use in other scripts
window.ClientSideDetector = ClientSideDetector;