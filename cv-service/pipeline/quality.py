"""Image quality checks: blur detection, resolution, item-in-frame."""

from __future__ import annotations

import cv2
import numpy as np

# Laplacian variance threshold for blur detection.
BLUR_THRESHOLD = 100.0

# Minimum shortest side in pixels.
MIN_SHORTEST_SIDE = 640

# Minimum percentage of image area that must be covered by the item mask.
MIN_ITEM_COVERAGE = 0.05


def check_quality(image_bytes: bytes) -> dict:
    """Run quality checks on a single image.

    Returns a dict with:
      - passed: bool — overall pass/fail
      - blur_score: float — Laplacian variance (higher = sharper)
      - blur_passed: bool
      - resolution_passed: bool
      - width: int
      - height: int
      - issues: list[str] — human-readable issue descriptions
    """
    arr = np.frombuffer(image_bytes, dtype=np.uint8)
    img = cv2.imdecode(arr, cv2.IMREAD_COLOR)
    if img is None:
        return {
            "passed": False,
            "blur_score": 0.0,
            "blur_passed": False,
            "resolution_passed": False,
            "width": 0,
            "height": 0,
            "issues": ["failed to decode image"],
        }

    h, w = img.shape[:2]
    issues: list[str] = []

    # Blur detection via Laplacian variance.
    gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
    blur_score = float(cv2.Laplacian(gray, cv2.CV_64F).var())
    blur_passed = blur_score >= BLUR_THRESHOLD
    if not blur_passed:
        issues.append(f"image is too blurry (score: {blur_score:.1f}, min: {BLUR_THRESHOLD})")

    # Resolution check.
    shortest = min(w, h)
    resolution_passed = shortest >= MIN_SHORTEST_SIDE
    if not resolution_passed:
        issues.append(
            f"resolution too low (shortest side: {shortest}px, min: {MIN_SHORTEST_SIDE}px)"
        )

    passed = blur_passed and resolution_passed

    return {
        "passed": passed,
        "blur_score": blur_score,
        "blur_passed": blur_passed,
        "resolution_passed": resolution_passed,
        "width": w,
        "height": h,
        "issues": issues,
    }
