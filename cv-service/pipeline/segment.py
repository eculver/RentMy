"""Item segmentation using SAM 2 (or fallback to passthrough).

The SAM 2 model is loaded lazily on first use. If the model weights are not
available, segmentation is a no-op and the normalized image is returned as-is.
This allows the cv-service to run in development without downloading large
model files.
"""

from __future__ import annotations

import logging
import cv2
import numpy as np

logger = logging.getLogger(__name__)

# SAM 2 model state (lazy-loaded singleton).
_sam_model = None
_sam_load_attempted = False


def _try_load_sam() -> bool:
    """Attempt to load SAM 2 model. Returns True if loaded successfully."""
    global _sam_model, _sam_load_attempted
    if _sam_load_attempted:
        return _sam_model is not None
    _sam_load_attempted = True

    try:
        # SAM 2 model loading would go here. For now, we operate in fallback
        # mode where segmentation returns the input image as-is. The model
        # weights need to be mounted at /app/models/sam2/.
        #
        # When SAM 2 is available:
        #   from segment_anything_2 import SAM2
        #   _sam_model = SAM2.from_pretrained("models/sam2/")
        logger.info("SAM 2 model not available, running in passthrough mode")
        return False
    except Exception:
        logger.warning("failed to load SAM 2 model, running in passthrough mode", exc_info=True)
        return False


def is_model_loaded() -> bool:
    """Check if the SAM 2 model is loaded."""
    _try_load_sam()
    return _sam_model is not None


def segment_item(image_bytes: bytes) -> bytes:
    """Segment the rental item from the background.

    If SAM 2 is available: produces an isolated item on transparent background (RGBA PNG).
    If SAM 2 is not available: returns the input image as-is (JPEG passthrough).
    """
    if not _try_load_sam():
        return image_bytes

    # When SAM 2 is available, this would:
    # 1. Decode image
    # 2. Run SAM 2 to produce a segmentation mask
    # 3. Apply mask to isolate the item
    # 4. Crop to bounding box
    # 5. Encode as PNG with alpha channel
    #
    # For now, passthrough:
    return image_bytes
