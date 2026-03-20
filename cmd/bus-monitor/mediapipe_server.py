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

def calculate_ear(eye_indices, landmarks, image_width, image_height):
    """Calculate Eye Aspect Ratio from eye landmarks"""
    coords = []
    for idx in eye_indices:
        if idx < len(landmarks):
            landmark = landmarks[idx]
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

def calculate_mar(landmarks, image_width, image_height):
    """
    Calculate Mouth Aspect Ratio (MAR) for yawn detection.
    MAR = vertical_distance / horizontal_distance
    Higher MAR indicates mouth open (yawning).
    """
    try:
        # Mouth landmark indices
        UPPER_LIP = 13
        LOWER_LIP = 14
        LEFT_CORNER = 61
        RIGHT_CORNER = 291
        
        # Get landmark coordinates
        upper = landmarks[UPPER_LIP]
        lower = landmarks[LOWER_LIP]
        left = landmarks[LEFT_CORNER]
        right = landmarks[RIGHT_CORNER]
        
        # Calculate vertical distance
        vertical = np.sqrt(
            (upper.x * image_width - lower.x * image_width)**2 + 
            (upper.y * image_height - lower.y * image_height)**2
        )
        
        # Calculate horizontal distance
        horizontal = np.sqrt(
            (left.x * image_width - right.x * image_width)**2 + 
            (left.y * image_height - right.y * image_height)**2
        )
        
        if horizontal == 0:
            return 0
        
        mar = vertical / horizontal
        return float(mar)
    except Exception as e:
        print(f"MAR calculation error: {e}")
        return 0

def calculate_head_pose(landmarks, image_width, image_height):
    """
    Calculate head pose (pitch, yaw, roll) using face landmarks.
    
    Returns:
        pitch: head nod (positive = looking down - drowsiness indicator)
        yaw: head turn (positive = looking right)
        roll: head tilt (positive = tilting right)
    """
    try:
        # Key facial landmarks for pose estimation
        NOSE_TIP = 1
        NOSE_BRIDGE = 6
        LEFT_EYE = 33
        RIGHT_EYE = 263
        LEFT_EAR = 234
        RIGHT_EAR = 454
        CHIN = 152
        FOREHEAD = 10
        
        # Get coordinates
        nose_tip = landmarks[NOSE_TIP]
        nose_bridge = landmarks[NOSE_BRIDGE]
        left_eye = landmarks[LEFT_EYE]
        right_eye = landmarks[RIGHT_EYE]
        chin = landmarks[CHIN]
        forehead = landmarks[FOREHEAD]
        
        # Convert to pixel coordinates
        nose_tip_x, nose_tip_y = nose_tip.x * image_width, nose_tip.y * image_height
        nose_bridge_x, nose_bridge_y = nose_bridge.x * image_width, nose_bridge.y * image_height
        left_eye_x, left_eye_y = left_eye.x * image_width, left_eye.y * image_height
        right_eye_x, right_eye_y = right_eye.x * image_width, right_eye.y * image_height
        chin_x, chin_y = chin.x * image_width, chin.y * image_height
        forehead_x, forehead_y = forehead.x * image_width, forehead.y * image_height
        
        # Calculate eye center
        eye_center_x = (left_eye_x + right_eye_x) / 2
        eye_center_y = (left_eye_y + right_eye_y) / 2
        
        # Roll: tilt around Z-axis (using eye positions)
        roll = np.degrees(np.arctan2(right_eye_y - left_eye_y, right_eye_x - left_eye_x))
        
        # Yaw: rotation around Y-axis (using nose position relative to eye center)
        nose_offset_x = nose_tip_x - eye_center_x
        yaw = np.degrees(np.arctan2(nose_offset_x, image_width / 2))
        
        # Pitch: rotation around X-axis (using nose and eye positions)
        nose_offset_y = nose_tip_y - eye_center_y
        face_height = chin_y - forehead_y
        if face_height != 0:
            pitch = np.degrees(np.arctan2(nose_offset_y, face_height))
        else:
            pitch = 0
        
        return float(pitch), float(yaw), float(roll)
    except Exception as e:
        print(f"Head pose error: {e}")
        return 0.0, 0.0, 0.0

def get_eye_bounding_box(eye_indices, landmarks, image_width, image_height):
    """Get bounding box for eye"""
    xs = [landmarks[i].x * image_width for i in eye_indices if i < len(landmarks)]
    ys = [landmarks[i].y * image_height for i in eye_indices if i < len(landmarks)]
    if not xs or not ys:
        return [0, 0, 0, 0]
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
                'mar': 0,
                'pitch': 0,
                'yaw': 0,
                'roll': 0,
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
                'mar': 0,
                'pitch': 0,
                'yaw': 0,
                'roll': 0,
                'face_box': [0, 0, 0, 0],
                'left_eye_box': [0, 0, 0, 0],
                'right_eye_box': [0, 0, 0, 0],
                'mouth_box': [0, 0, 0, 0]
            })
        
        # Get face bounding box (from all landmarks)
        all_xs = [lm.x * w for lm in landmarks]
        all_ys = [lm.y * h for lm in landmarks]
        face_box = [int(min(all_xs)), int(min(all_ys)), int(max(all_xs)), int(max(all_ys))]
        
        # Calculate EAR
        try:
            left_ear = calculate_ear(LEFT_EYE, landmarks, w, h)
            right_ear = calculate_ear(RIGHT_EYE, landmarks, w, h)
        except Exception as e:
            print(f"EAR calculation error: {e}")
            left_ear = 0
            right_ear = 0
        avg_ear = (left_ear + right_ear) / 2.0
        
        # Calculate MAR
        try:
            mar = calculate_mar(landmarks, w, h)
        except Exception as e:
            print(f"MAR calculation error: {e}")
            mar = 0
        
        # Calculate head pose
        try:
            pitch, yaw, roll = calculate_head_pose(landmarks, w, h)
            print(f"[DEBUG] Head pose - Pitch: {pitch:.1f}, Yaw: {yaw:.1f}, Roll: {roll:.1f}")
        except Exception as e:
            print(f"Head pose error: {e}")
            pitch, yaw, roll = 0.0, 0.0, 0.0
        
        # Get eye bounding boxes
        print(f"[DEBUG] LEFT_EYE={LEFT_EYE}, landmarks length={len(landmarks)}")
        try:
            left_eye_box = get_eye_bounding_box(LEFT_EYE, landmarks, w, h)
            right_eye_box = get_eye_bounding_box(RIGHT_EYE, landmarks, w, h)
            print(f"[DEBUG] Eye boxes - Left: {left_eye_box}, Right: {right_eye_box}")
        except Exception as e:
            print(f"[ERROR] Eye box error: {e}")
            import traceback
            traceback.print_exc()
            left_eye_box = [0, 0, 0, 0]
            right_eye_box = [0, 0, 0, 0]
        
        # Get mouth bounding box
        try:
            mouth_box = get_mouth_bounding_box(landmarks, w, h)
            print(f"[DEBUG] Mouth box: {mouth_box}")
        except Exception as e:
            print(f"[ERROR] Mouth box error: {e}")
            import traceback
            traceback.print_exc()
            mouth_box = [0, 0, 0, 0]
        
        # Get eye coordinates for visualization
        try:
            left_eye_coords = [[int(landmarks[i].x * w), int(landmarks[i].y * h)] for i in LEFT_EYE]
            right_eye_coords = [[int(landmarks[i].x * w), int(landmarks[i].y * h)] for i in RIGHT_EYE]
        except Exception as e:
            print(f"Eye coords error: {e}")
            left_eye_coords = []
            right_eye_coords = []
        
        # Get mouth landmark coordinates for visualization
        try:
            mouth_indices = [13, 14, 61, 78, 291, 308]  # Key mouth points
            mouth_coords = [[int(landmarks[i].x * w), int(landmarks[i].y * h)] for i in mouth_indices if i < len(landmarks)]
        except Exception as e:
            print(f"Mouth coords error: {e}")
            mouth_coords = []
        
        return jsonify({
            'face_detected': True,
            'ear': float(avg_ear),
            'left_ear': float(left_ear),
            'right_ear': float(right_ear),
            'mar': float(mar),
            'pitch': float(pitch),
            'yaw': float(yaw),
            'roll': float(roll),
            'mar': float(mar),
            'face_box': face_box,
            'left_eye_box': left_eye_box,
            'right_eye_box': right_eye_box,
            'mouth_box': mouth_box,
            'left_eye': left_eye_coords,
            'right_eye': right_eye_coords,
            'mouth': mouth_coords
        })
        
    except Exception as e:
        return jsonify({'error': f'Server error: {str(e)}'}), 500

@app.route('/health', methods=['GET'])
def health():
    return jsonify({'status': 'ok'})

if __name__ == '__main__':
    print("Starting MediaPipe face landmark server on port 5000...")
    app.run(host='0.0.0.0', port=5000, threaded=True)
