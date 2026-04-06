"""CV Service — FastAPI sidecar for photo diff preprocessing.

Provides three endpoints:
  POST /preprocess  — normalize, segment (if SAM available), and pair check-in/out photos
  POST /quality     — run quality checks on a single image
  GET  /health      — liveness + readiness probe
"""

from __future__ import annotations

import io
import logging
from typing import Any

from fastapi import FastAPI, HTTPException, UploadFile, File, Form
from fastapi.responses import JSONResponse
import numpy as np

from pipeline.normalize import normalize_image
from pipeline.segment import segment_item, is_model_loaded
from pipeline.match_angles import match_pairs
from pipeline.quality import check_quality

logger = logging.getLogger("cv-service")
logging.basicConfig(level=logging.INFO)

app = FastAPI(title="RentMy CV Service", version="1.0.0")


@app.get("/health")
async def health() -> dict[str, Any]:
    return {"status": "ok", "model_loaded": is_model_loaded()}


@app.post("/quality")
async def quality_check(image: UploadFile = File(...)) -> dict[str, Any]:
    """Run quality checks on a single image: blur, resolution, item-in-frame."""
    image_bytes = await image.read()
    if not image_bytes:
        raise HTTPException(status_code=400, detail="empty image")
    result = check_quality(image_bytes)
    return result


@app.post("/preprocess")
async def preprocess(
    checkin_count: int = Form(...),
    checkout_count: int = Form(...),
    orientations_json: str = Form(default="[]"),
    checkin_images: list[UploadFile] = File(...),
    checkout_images: list[UploadFile] = File(...),
) -> dict[str, Any]:
    """Preprocess check-in and check-out photo sets.

    Steps:
    1. Normalize all images (resize, CLAHE histogram equalization, white balance)
    2. Segment items from backgrounds (if SAM model is loaded)
    3. Match check-in/check-out pairs by orientation metadata

    Returns base64-encoded paired, processed image crops.
    """
    import base64
    import json

    if len(checkin_images) != checkin_count:
        raise HTTPException(
            status_code=400,
            detail=f"expected {checkin_count} check-in images, got {len(checkin_images)}",
        )
    if len(checkout_images) != checkout_count:
        raise HTTPException(
            status_code=400,
            detail=f"expected {checkout_count} check-out images, got {len(checkout_images)}",
        )

    try:
        orientations = json.loads(orientations_json)
    except json.JSONDecodeError:
        raise HTTPException(status_code=400, detail="invalid orientations JSON")

    # Read all images.
    checkin_bytes: list[bytes] = []
    for img in checkin_images:
        data = await img.read()
        if not data:
            raise HTTPException(status_code=400, detail="empty check-in image")
        checkin_bytes.append(data)

    checkout_bytes: list[bytes] = []
    for img in checkout_images:
        data = await img.read()
        if not data:
            raise HTTPException(status_code=400, detail="empty check-out image")
        checkout_bytes.append(data)

    # Step 1: Normalize all images.
    checkin_normalized = [normalize_image(b) for b in checkin_bytes]
    checkout_normalized = [normalize_image(b) for b in checkout_bytes]

    # Step 2: Segment items (isolate from background).
    checkin_segmented = [segment_item(b) for b in checkin_normalized]
    checkout_segmented = [segment_item(b) for b in checkout_normalized]

    # Step 3: Match pairs by orientation.
    checkin_orientations = []
    checkout_orientations = []
    for o in orientations:
        if o.get("type") == "checkin":
            checkin_orientations.append(o)
        elif o.get("type") == "checkout":
            checkout_orientations.append(o)

    pairs = match_pairs(checkin_orientations, checkout_orientations)
    if not pairs:
        # Fallback: pair by index order.
        n = min(len(checkin_segmented), len(checkout_segmented))
        pairs = [(i, i) for i in range(n)]

    # Build response with base64-encoded paired images.
    paired_results: list[dict[str, str]] = []
    for ci_idx, co_idx in pairs:
        if ci_idx < len(checkin_segmented) and co_idx < len(checkout_segmented):
            paired_results.append({
                "checkin_image": base64.b64encode(checkin_segmented[ci_idx]).decode(),
                "checkout_image": base64.b64encode(checkout_segmented[co_idx]).decode(),
                "checkin_index": ci_idx,
                "checkout_index": co_idx,
            })

    return {
        "pairs": paired_results,
        "total_checkin": len(checkin_segmented),
        "total_checkout": len(checkout_segmented),
        "pairs_matched": len(paired_results),
    }
