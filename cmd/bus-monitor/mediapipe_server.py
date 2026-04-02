#!/usr/bin/env python3
"""
MediaPipe Face Landmark Detection Server for Drowsiness Detection
==================================================================
- Real-time face landmark detection using MediaPipe
- Eye Aspect Ratio (EAR) for eye closure monitoring  
- Head pose estimation (pitch, yaw, roll)
- Yawn detection via Mouth Aspect Ratio (MAR)
- Supports Flask API for GoCV integration
"""

import cv2
import numpy as np
import time
import sys
import logging
import io
import base64
from flask import Flask, request, jsonify, make_response
from PIL import Image as PILImage

# MediaPipe imports
from mediapipe.tasks import python
from mediapipe.tasks.python import vision
from mediapipe.tasks.python.vision.core import image as mp_image
from mediapipe import Image

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# =============================================================================
# CONFIGURATION
# =============================================================================

# Model path
MODEL_PATH = '/home/sathuku/psv-crowd-counter/internal/core/models/face_landmarker.task'

# EAR (Eye Aspect Ratio) configuration
EAR_THRESHOLD = 0.25  # Below this = eye closed

# Head pose configuration  
PITCH_THRESHOLD = 20.0  # Head nod threshold
YAW_THRESHOLD = 25.0   # Head turn threshold  
ROLL_THRESHOLD = 20.0  # Head tilt threshold

# MAR (Mouth Aspect Ratio) for yawning
MAR_THRESHOLD = 0.5

# =============================================================================
# LANDMARK INDICES
# =============================================================================

# Left eye landmarks (MediaPipe 468-point mesh)
LEFT_EYE = [33, 159, 158, 133, 153, 145]
# Right eye landmarks
RIGHT_EYE = [362, 386, 387, 263, 380, 374]
# Nose landmarks for head pose
NOSE_INDICES = [1, 2, 3, 4, 5]  # nose tip and surrounding points

# =============================================================================
# FLASK SERVER SETUP
# =============================================================================

app = Flask(__name__)

# Initialize MediaPipe Face Landmarker
logger.info("Loading MediaPipe face landmarker model...")
try:
    base_options = python.BaseOptions(model_asset_path=MODEL_PATH)
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
    logger.info("Model loaded successfully")
except Exception as e:
    logger.error(f"Failed to load model: {e}")
    detector = None

# =============================================================================
# DETECTION FUNCTIONS
# =============================================================================

def calculate_ear(landmarks, eye_indices, image_width, image_height):
    """
    Calculate Eye Aspect Ratio from eye landmarks.
    
    EAR = (A + B) / (2 * C)
    - A: vertical distance from top-left to bottom-left
    - B: vertical distance from top-right to bottom-right
    - C: horizontal distance from left to right corner
    """
    try:
        coords = []
        for idx in eye_indices:
            if idx >= len(landmarks):
                return 0
            landmark = landmarks[idx]
            coords.append((landmark.x * image_width, landmark.y * image_height))
        
        if len(coords) < 6:
            return 0
        
        # Vertical distances
        A = np.sqrt((coords[1][0] - coords[5][0])**2 + (coords[1][1] - coords[5][1])**2)
        B = np.sqrt((coords[2][0] - coords[4][0])**2 + (coords[2][1] - coords[4][1])**2)
        # Horizontal distance
        C = np.sqrt((coords[0][0] - coords[3][0])**2 + (coords[0][1] - coords[3][1])**2)
        
        if C == 0:
            return 0
        
        ear = (A + B) / (2.0 * C)
        return float(ear)
    except:
        return 0


def calculate_mar(landmarks, image_width, image_height):
    """Calculate Mouth Aspect Ratio for yawn detection"""
    try:
        UPPER_LIP = 13
        LOWER_LIP = 14
        LEFT_CORNER = 61
        RIGHT_CORNER = 291
        
        upper = landmarks[UPPER_LIP]
        lower = landmarks[LOWER_LIP]
        vertical = np.sqrt((upper.x * image_width - lower.x * image_width)**2 + 
                          (upper.y * image_height - lower.y * image_height)**2)
        
        left = landmarks[LEFT_CORNER]
        right = landmarks[RIGHT_CORNER]
        horizontal = np.sqrt((left.x * image_width - right.x * image_width)**2 + 
                            (left.y * image_height - right.y * image_height)**2)
        
        if horizontal == 0:
            return 0
        
        mar = vertical / horizontal
        return float(mar)
    except:
        return 0


def calculate_head_pose(landmarks, image_width, image_height):
    """
    Calculate head pose angles (pitch, yaw, roll) in degrees.
    
    - Pitch: Head nod (positive = looking down)
    - Yaw: Head turn (positive = looking right)
    - Roll: Head tilt (positive = tilting right)
    """
    try:
        # Key landmarks
        nose_tip = landmarks[1]     # Nose tip
        chin = landmarks[152]        # Chin
        left_eye_left = landmarks[33]   # Left eye outer corner
        left_eye_right = landmarks[133]  # Left eye inner corner
        right_eye_left = landmarks[362]  # Right eye inner corner
        right_eye_right = landmarks[263] # Right eye outer corner
        forehead = landmarks[10]      # Forehead
        
        # Convert to pixel coordinates
        nose = (nose_tip.x * image_width, nose_tip.y * image_height)
        chin_pos = (chin.x * image_width, chin.y * image_height)
        
        left_eye_center = ((left_eye_left.x + left_eye_right.x) / 2 * image_width,
                         (left_eye_left.y + left_eye_right.y) / 2 * image_height)
        right_eye_center = ((right_eye_left.x + right_eye_right.x) / 2 * image_width,
                           (right_eye_left.y + right_eye_right.y) / 2 * image_height)
        
        # Eye midpoint
        eye_mid_x = (left_eye_center[0] + right_eye_center[0]) / 2
        eye_mid_y = (left_eye_center[1] + right_eye_center[1]) / 2
        
        # Eye distance for normalization
        eye_distance = right_eye_center[0] - left_eye_center[0]
        
        # Pitch: up/down (based on nose relative to eye line)
        if eye_distance > 0:
            pitch = np.degrees(np.arctan2(nose[1] - eye_mid_y, eye_distance * 0.5))
        else:
            pitch = 0.0
        
        # Yaw: left/right (based on nose deviation from eye midpoint)
        if eye_distance > 0:
            yaw = np.degrees(np.arctan2(nose[0] - eye_mid_x, eye_distance * 0.5))
        else:
            yaw = 0.0
        
        # Roll: tilt (based on angle of eye line from horizontal)
        roll = np.degrees(np.arctan2(
            right_eye_center[1] - left_eye_center[1],
            right_eye_center[0] - left_eye_center[0]
        ))
        
        return float(pitch), float(yaw), float(roll)
    except:
        return 0.0, 0.0, 0.0


def get_eye_bounding_box(eye_indices, landmarks, image_width, image_height):
    """Get bounding box for an eye"""
    try:
        xs = [landmarks[i].x * image_width for i in eye_indices if i < len(landmarks)]
        ys = [landmarks[i].y * image_height for i in eye_indices if i < len(landmarks)]
        
        if not xs or not ys:
            return [0, 0, 0, 0]
        
        min_x, max_x = int(min(xs)), int(max(xs))
        min_y, max_y = int(min(ys)), int(max(ys))
        
        # Add small padding
        pad = 3
        return [max(0, min_x - pad), max(0, min_y - pad), 
                min(image_width, max_x + pad), min(image_height, max_y + pad)]
    except:
        return [0, 0, 0, 0]


def get_mouth_bounding_box(landmarks, image_width, image_height):
    """Get bounding box for mouth"""
    try:
        mouth_indices = [13, 14, 61, 291, 78, 308]  # Mouth corner and lip points
        xs = [landmarks[i].x * image_width for i in mouth_indices if i < len(landmarks)]
        ys = [landmarks[i].y * image_height for i in mouth_indices if i < len(landmarks)]
        
        if not xs or not ys:
            return [0, 0, 0, 0]
        
        min_x, max_x = int(min(xs)), int(max(xs))
        min_y, max_y = int(min(ys)), int(max(ys))
        
        pad = 2
        return [max(0, min_x - pad), max(0, min_y - pad),
                min(image_width, max_x + pad), min(image_height, max_y + pad)]
    except:
        return [0, 0, 0, 0]


def get_face_bounding_box(landmarks, image_width, image_height):
    """Get bounding box for entire face"""
    try:
        if len(landmarks) < 468:
            return [0, 0, 0, 0]
        
        xs = [lm.x * image_width for lm in landmarks]
        ys = [lm.y * image_height for lm in landmarks]
        
        min_x, max_x = int(min(xs)), int(max(xs))
        min_y, max_y = int(min(ys)), int(max(ys))
        
        return [min_x, min_y, max_x, max_y]
    except:
        return [0, 0, 0, 0]


def get_nose_coordinates(landmarks, image_width, image_height):
    """Get nose landmark coordinates"""
    try:
        nose_indices = [1, 2, 3, 4, 5, 6]  # Nose tip and surrounding
        coords = []
        for idx in nose_indices:
            if idx < len(landmarks):
                x = int(landmarks[idx].x * image_width)
                y = int(landmarks[idx].y * image_height)
                coords.append([x, y])
        return coords
    except:
        return []


# =============================================================================
# FLASK ROUTES
# =============================================================================

@app.after_request
def add_cors_headers(response):
    """Add CORS headers to all responses"""
    response.headers['Access-Control-Allow-Origin'] = '*'
    response.headers['Access-Control-Allow-Headers'] = 'Content-Type,Authorization'
    response.headers['Access-Control-Allow-Methods'] = 'GET,POST,OPTIONS'
    return response

@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint"""
    return jsonify({
        'status': 'healthy',
        'service': 'mediapipe-face-landmarker',
        'model_loaded': detector is not None
    })

@app.route('/detect', methods=['POST'])
def detect():
    """Main detection endpoint - returns landmarks and metrics for GoCV"""
    print("[DEBUG] Received detect request")
    
    if detector is None:
        return jsonify({'error': 'Model not loaded'}), 500
    
    try:
        if 'image' not in request.json:
            return jsonify({'error': 'No image provided'}), 400
        
        # Decode base64 image
        try:
            image_data = request.json['image']
            image_bytes = base64.b64decode(image_data)
        except Exception as e:
            print(f"[ERROR] Failed to decode image: {e}")
            return jsonify({'error': f'Failed to decode image: {str(e)}'}), 400
        
        # Convert to OpenCV format
        try:
            pil_img = PILImage.open(io.BytesIO(image_bytes))
            img_array = np.array(pil_img)
        except Exception as e:
            print(f"[ERROR] Failed to process image: {e}")
            return jsonify({'error': f'Failed to process image: {str(e)}'}), 400
        
        # Handle different color formats
        try:
            if len(img_array.shape) == 2:
                # Grayscale
                img_rgb = cv2.cvtColor(img_array, cv2.COLOR_GRAY2RGB)
            elif len(img_array.shape) == 3 and img_array.shape[2] == 4:
                # RGBA
                img_rgb = cv2.cvtColor(img_array, cv2.COLOR_RGBA2RGB)
            elif len(img_array.shape) == 3 and img_array.shape[2] == 3:
                # BGR (OpenCV) - convert to RGB
                img_rgb = cv2.cvtColor(img_array, cv2.COLOR_BGR2RGB)
            else:
                img_rgb = img_array
        except Exception as e:
            print(f"[ERROR] Color conversion failed: {e}")
            img_rgb = img_array
            return jsonify({'error': f'Color conversion failed: {str(e)}'}), 400
        
        h, w = img_rgb.shape[:2]
        
        if h == 0 or w == 0:
            return jsonify({'error': 'Invalid image dimensions'}), 400
        
        # Create MediaPipe Image
        try:
            mp_img = Image(image_format=mp_image.ImageFormat.SRGB, data=img_rgb)
        except Exception as e:
            print(f"[ERROR] Failed to create MediaPipe image: {e}")
            return jsonify({'error': f'Failed to create image: {str(e)}'}), 400
        
        # Detect landmarks
        try:
            result = detector.detect(mp_img)
        except Exception as e:
            print(f"[ERROR] Detection failed: {e}")
            return jsonify({'error': f'Detection failed: {str(e)}'}), 500
        
        # No face detected
        if not result or not result.face_landmarks:
            print("[DEBUG] No face detected")
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
                'mouth_box': [0, 0, 0, 0],
                'left_eye': [],
                'right_eye': [],
                'nose': []
            })
        
        landmarks = result.face_landmarks[0]
        
        # Validate landmarks
        if len(landmarks) < 468:
            print(f"[DEBUG] Warning: Not enough landmarks: {len(landmarks)}")
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
                'mouth_box': [0, 0, 0, 0],
                'left_eye': [],
                'right_eye': [],
                'nose': []
            })
        
        # Calculate metrics
        try:
            left_ear = calculate_ear(landmarks, LEFT_EYE, w, h)
            right_ear = calculate_ear(landmarks, RIGHT_EYE, w, h)
            avg_ear = (left_ear + right_ear) / 2.0
        except Exception as e:
            print(f"[ERROR] EAR calculation error: {e}")
            left_ear = right_ear = avg_ear = 0
            return jsonify({'error': f'EAR calculation error: {str(e)}'}), 500
        
        try:
            mar = calculate_mar(landmarks, w, h)
        except Exception as e:
            print(f"[ERROR] MAR calculation error: {e}")
            mar = 0
            return jsonify({'error': f'MAR calculation error: {str(e)}'}), 500
        
        try:
            pitch, yaw, roll = calculate_head_pose(landmarks, w, h)
        except Exception as e:
            print(f"[ERROR] Head pose error: {e}")
            pitch = yaw = roll = 0
            return jsonify({'error': f'Head pose error: {str(e)}'}), 500
        
        # Get bounding boxes
        try:
            face_box = get_face_bounding_box(landmarks, w, h)
            left_eye_box = get_eye_bounding_box(LEFT_EYE, landmarks, w, h)
            right_eye_box = get_eye_bounding_box(RIGHT_EYE, landmarks, w, h)
            mouth_box = get_mouth_bounding_box(landmarks, w, h)
        except Exception as e:
            print(f"[ERROR] Bounding box error: {e}")
            face_box = left_eye_box = right_eye_box = mouth_box = [0, 0, 0, 0]
            return jsonify({'error': f'Bounding box error: {str(e)}'}), 500
        
        # Get eye landmark coordinates
        try:
            left_eye_coords = [[int(landmarks[i].x * w), int(landmarks[i].y * h)] for i in LEFT_EYE]
            right_eye_coords = [[int(landmarks[i].x * w), int(landmarks[i].y * h)] for i in RIGHT_EYE]
        except Exception as e:
            print(f"[ERROR] Eye coords error: {e}")
            left_eye_coords = right_eye_coords = []
            return jsonify({'error': f'Eye coords error: {str(e)}'}), 500
        
        # Get nose coordinates
        try:
            nose_coords = get_nose_coordinates(landmarks, w, h)
        except Exception as e:
            print(f"[ERROR] Nose coords error: {e}")
            nose_coords = []
            return jsonify({'error': f'Nose coords error: {str(e)}'}), 500
        
        # Print debug info to terminal
        print(f"[INFO] Face detected! EAR: {avg_ear:.3f}, MAR: {mar:.3f}, Pitch: {pitch:.1f}°, Yaw: {yaw:.1f}°, Roll: {roll:.1f}°")
        
        # Print alerts if detected
        if avg_ear < EAR_THRESHOLD:
            print(f"[ALERT] Eye closure detected! EAR={avg_ear:.3f} < {EAR_THRESHOLD}")
        
        if abs(pitch) > PITCH_THRESHOLD:
            print(f"[ALERT] Head pitch detected! Pitch={pitch:.1f}°")
        
        if abs(yaw) > YAW_THRESHOLD:
            print(f"[ALERT] Head yaw detected! Yaw={yaw:.1f}°")
        
        if abs(roll) > ROLL_THRESHOLD:
            print(f"[ALERT] Head roll detected! Roll={roll:.1f}°")
        
        if mar > MAR_THRESHOLD:
            print(f"[ALERT] Yawning detected! MAR={mar:.3f} > {MAR_THRESHOLD}")
        
        # Return JSON response matching Go expectations
        return jsonify({
            'face_detected': True,
            'ear': float(avg_ear),
            'left_ear': float(left_ear),
            'right_ear': float(right_ear),
            'mar': float(mar),
            'pitch': float(pitch),
            'yaw': float(yaw),
            'roll': float(roll),
            'face_box': face_box,
            'left_eye_box': left_eye_box,
            'right_eye_box': right_eye_box,
            'mouth_box': mouth_box,
            'left_eye': left_eye_coords,
            'right_eye': right_eye_coords,
            'nose': nose_coords
        })
        
    except Exception as e:
        print(f"[ERROR] Server error: {e}")
        import traceback
        traceback.print_exc()
        return jsonify({'error': f'Server error: {str(e)}'}), 500


if __name__ == '__main__':
    logger.info("Starting MediaPipe Face Landmark Detection Server on port 5000")
    app.run(host='0.0.0.0', port=5000, debug=False, threaded=True)
