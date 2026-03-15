#!/usr/bin/env python3
"""
MediaPipe Face Landmark Detection Server for Drowsiness Detection
Using MediaPipe 2.0 API
"""

import cv2
import numpy as np
from flask import Flask, request, jsonify
import base64
import io
from PIL import Image as PILImage
import logging

# MediaPipe imports
from mediapipe.tasks import python
from mediapipe.tasks.python import vision
from mediapipe.tasks.python.vision.core import image as mp_image
from mediapipe import Image

# Disable Flask logging
log = logging.getLogger('werkzeug')
log.setLevel(logging.ERROR)

# Initialize Flask
app = Flask(__name__)

# Eye landmark indices for EAR calculation (MediaPipe 468-point face mesh)
# Left eye: indices 33, 133, 160, 158, 153, 144
# Right eye: indices 362, 263, 387, 373, 380, 385
LEFT_EYE = [33, 133, 160, 158, 153, 144]
RIGHT_EYE = [362, 263, 387, 373, 380, 385]
# Mouth landmarks - using outer lip landmarks
MOUTH = [13, 78, 308, 61]

# Create face landmarker
base_options = python.BaseOptions(model_asset_path='/home/sathuku/psv-crowd-counter/internal/core/models/face_landmarker.task')
options = vision.FaceLandmarkerOptions(
    base_options=base_options,
    running_mode=vision.RunningMode.IMAGE,
    num_faces=1,
    min_face_detection_confidence=0.3,
    min_face_presence_confidence=0.3,
    min_tracking_confidence=0.3,
    output_face_blendshapes=False,
    output_facial_transformation_matrixes=False
)
detector = vision.FaceLandmarker.create_from_options(options)

def calculate_ear(eye_landmarks, image_width, image_height):
    """Calculate Eye Aspect Ratio from eye landmarks"""
    coords = []
    for idx in eye_landmarks:
        landmark = eye_landmarks[idx]
        coords.append((landmark.x * image_width, landmark.y * image_height))
    
    if len(coords) < 6:
        return 0
    
    # EAR calculation
    A = np.sqrt((coords[1][0] - coords[5][0])**2 + (coords[1][1] - coords[5][1])**2)
    B = np.sqrt((coords[2][0] - coords[4][0])**2 + (coords[2][1] - coords[4][1])**2)
    C = np.sqrt((coords[0][0] - coords[3][0])**2 + (coords[0][1] - coords[3][1])**2)
    
    if C == 0:
        return 0
    
    ear = (A + B) / (2.0 * C)
    return ear

def get_eye_bounding_box(eye_landmarks, image_width, image_height):
    """Get bounding box for eye"""
    xs = [eye_landmarks[i].x * image_width for i in eye_landmarks[:6]]
    ys = [eye_landmarks[i].y * image_height for i in eye_landmarks[:6]]
    min_x, max_x = int(min(xs)), int(max(xs))
    min_y, max_y = int(min(ys)), int(max(ys))
    # Add padding
    pad = 3
    return [min_x - pad, min_y - pad, max_x + pad, max_y + pad]

def get_mouth_bounding_box(landmarks, image_width, image_height):
    """Get bounding box for mouth using outer lip landmarks"""
    # Use indices 13 (upper lip), 14 (lower lip), 78 (right corner), 308 (left corner)
    mouth_indices = [13, 78, 308, 61]  # Simple outer mouth box
    xs = [landmarks[i].x * image_width for i in mouth_indices if i < len(landmarks)]
    ys = [landmarks[i].y * image_height for i in mouth_indices if i < len(landmarks)]
    if not xs or not ys:
        return [0, 0, 0, 0]
    min_x, max_x = int(min(xs)), int(max(xs))
    min_y, max_y = int(min(ys)), int(max(ys))
    return [min_x, min_y, max_x, max_y]

@app.route('/detect', methods=['POST'])
def detect():
    """Detect face landmarks and calculate EAR"""
    print("Received detect request")
    try:
        if 'image' not in request.json:
            print("No image in request")
            return jsonify({'error': 'No image provided'}), 400
        
        # Decode base64 image
        try:
            image_data = request.json['image']
            image_bytes = base64.b64decode(image_data)
            print(f"Decoded image bytes: {len(image_bytes)}")
        except Exception as e:
            print(f"Failed to decode image: {e}")
            return jsonify({'error': f'Failed to decode image: {str(e)}'}), 400
        
        # Convert to OpenCV format
        try:
            pil_img = PILImage.open(io.BytesIO(image_bytes))
            img_array = np.array(pil_img)
        except Exception as e:
            return jsonify({'error': f'Failed to process image: {str(e)}'}), 400
        
        # Handle different image formats
        try:
            if len(img_array.shape) == 2:
                # Grayscale - convert to RGB
                img_rgb = cv2.cvtColor(img_array, cv2.COLOR_GRAY2RGB)
            elif len(img_array.shape) == 3 and img_array.shape[2] == 4:
                # RGBA
                img_rgb = cv2.cvtColor(img_array, cv2.COLOR_RGBA2RGB)
            elif len(img_array.shape) == 3 and img_array.shape[2] == 3:
                # BGR from OpenCV/GoCV - convert to RGB
                img_rgb = cv2.cvtColor(img_array, cv2.COLOR_BGR2RGB)
            else:
                # Unknown format, try to convert
                img_rgb = cv2.cvtColor(img_array, cv2.COLOR_BGR2RGB)
        except Exception as e:
            return jsonify({'error': f'Failed to convert color: {str(e)}'}), 400
        
        h, w = img_rgb.shape[:2]
        
        if h == 0 or w == 0:
            return jsonify({'error': 'Invalid image dimensions'}), 400
        
        # Create MediaPipe Image with correct format
        try:
            mp_img = Image(image_format=mp_image.ImageFormat.SRGB, data=img_rgb)
        except Exception as e:
            return jsonify({'error': f'Failed to create MediaPipe image: {str(e)}'}), 400
        
        # Detect landmarks
        try:
            result = detector.detect(mp_img)
            print(f"Detection result: {result}")
        except Exception as e:
            return jsonify({'error': f'Failed to detect landmarks: {str(e)}'}), 500
        
        if not result or not result.face_landmarks:
            print("No face landmarks detected")
            return jsonify({
                'face_detected': False,
                'ear': 0,
                'left_ear': 0,
                'right_ear': 0,
                'face_box': [0, 0, 0, 0],
                'left_eye_box': [0, 0, 0, 0],
                'right_eye_box': [0, 0, 0, 0],
                'mouth_box': [0, 0, 0, 0]
            })
        
        landmarks = result.face_landmarks[0]
        
        # Validate landmarks have enough elements
        if len(landmarks) < 468:
            print(f"Warning: Not enough landmarks: {len(landmarks)}")
            return jsonify({
                'face_detected': False,
                'ear': 0,
                'left_ear': 0,
                'right_ear': 0,
                'face_box': [0, 0, 0, 0],
                'left_eye_box': [0, 0, 0, 0],
                'right_eye_box': [0, 0, 0, 0],
                'mouth_box': [0, 0, 0, 0]
            })
        
        # Get face bounding box (from all landmarks)
        all_xs = [lm.x * w for lm in landmarks]
        all_ys = [lm.y * h for lm in landmarks]
        face_box = [int(min(all_xs)), int(min(all_ys)), int(max(all_xs)), int(max(all_ys))]
        
        # Calculate EAR - use first 6 indices only
        try:
            left_ear = calculate_ear(LEFT_EYE, w, h)
            right_ear = calculate_ear(RIGHT_EYE, w, h)
        except Exception as e:
            print(f"EAR calculation error: {e}")
            left_ear = 0
            right_ear = 0
        avg_ear = (left_ear + right_ear) / 2.0
        
        # Get eye bounding boxes
        try:
            left_eye_box = get_eye_bounding_box(LEFT_EYE, w, h)
            right_eye_box = get_eye_bounding_box(RIGHT_EYE, w, h)
        except Exception as e:
            print(f"Eye box error: {e}")
            left_eye_box = [0, 0, 0, 0]
            right_eye_box = [0, 0, 0, 0]
        
        # Get mouth bounding box
        try:
            mouth_box = get_mouth_bounding_box(landmarks, w, h)
        except Exception as e:
            print(f"Mouth box error: {e}")
            mouth_box = [0, 0, 0, 0]
        
        # Get eye coordinates for visualization
        try:
            left_eye_coords = [(int(landmarks[i].x * w), int(landmarks[i].y * h)) for i in LEFT_EYE]
            right_eye_coords = [(int(landmarks[i].x * w), int(landmarks[i].y * h)) for i in RIGHT_EYE]
        except Exception as e:
            print(f"Eye coords error: {e}")
            left_eye_coords = []
            right_eye_coords = []
        
        return jsonify({
            'face_detected': True,
            'ear': float(avg_ear),
            'left_ear': float(left_ear),
            'right_ear': float(right_ear),
            'face_box': face_box,
            'left_eye_box': left_eye_box,
            'right_eye_box': right_eye_box,
            'mouth_box': mouth_box,
            'left_eye': left_eye_coords,
            'right_eye': right_eye_coords
        })
        
    except Exception as e:
        return jsonify({'error': f'Server error: {str(e)}'}), 500

@app.route('/health', methods=['GET'])
def health():
    return jsonify({'status': 'ok'})

if __name__ == '__main__':
    print("Starting MediaPipe face landmark server on port 5000...")
    app.run(host='0.0.0.0', port=5000, threaded=True)
