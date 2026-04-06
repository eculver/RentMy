"""Image normalization: resize + CLAHE histogram equalization + white balance."""

from __future__ import annotations

import cv2
import numpy as np

# Resize so the longest side is 1024 pixels.
MAX_LONGEST_SIDE = 1024


def normalize_image(image_bytes: bytes) -> bytes:
    """Normalize an image for consistent comparison.

    Steps:
    1. Decode the image from bytes.
    2. Resize so longest side is MAX_LONGEST_SIDE.
    3. Apply CLAHE on the L channel of LAB color space for lighting normalization.
    4. Simple white balance via gray-world assumption.
    5. Re-encode as JPEG.
    """
    arr = np.frombuffer(image_bytes, dtype=np.uint8)
    img = cv2.imdecode(arr, cv2.IMREAD_COLOR)
    if img is None:
        return image_bytes  # Return original if decode fails.

    # Resize.
    h, w = img.shape[:2]
    longest = max(h, w)
    if longest > MAX_LONGEST_SIDE:
        scale = MAX_LONGEST_SIDE / longest
        new_w = int(w * scale)
        new_h = int(h * scale)
        img = cv2.resize(img, (new_w, new_h), interpolation=cv2.INTER_AREA)

    # CLAHE on LAB L-channel.
    lab = cv2.cvtColor(img, cv2.COLOR_BGR2LAB)
    l_channel, a_channel, b_channel = cv2.split(lab)
    clahe = cv2.createCLAHE(clipLimit=2.0, tileGridSize=(8, 8))
    l_channel = clahe.apply(l_channel)
    lab = cv2.merge([l_channel, a_channel, b_channel])
    img = cv2.cvtColor(lab, cv2.COLOR_LAB2BGR)

    # Gray-world white balance.
    img = _gray_world_wb(img)

    _, encoded = cv2.imencode(".jpg", img, [cv2.IMWRITE_JPEG_QUALITY, 95])
    return encoded.tobytes()


def _gray_world_wb(img: np.ndarray) -> np.ndarray:
    """Apply gray-world white balance correction."""
    result = img.astype(np.float32)
    avg_b = np.mean(result[:, :, 0])
    avg_g = np.mean(result[:, :, 1])
    avg_r = np.mean(result[:, :, 2])
    avg_all = (avg_b + avg_g + avg_r) / 3.0

    if avg_b > 0:
        result[:, :, 0] *= avg_all / avg_b
    if avg_g > 0:
        result[:, :, 1] *= avg_all / avg_g
    if avg_r > 0:
        result[:, :, 2] *= avg_all / avg_r

    return np.clip(result, 0, 255).astype(np.uint8)
