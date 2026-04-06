# Commit 0c5d226 — Photo Diff Pipeline

## What
Two-stage photo diff pipeline: Python CV sidecar + Go service + LLM integration.

## Why
PRD §11.1 requires a photo diff pipeline as the evidentiary backbone of the dispute system. This is a Phase 6 prerequisite — DisputeAgent (6.2) depends on photo diff results.

## Key decisions
- **Python sidecar over Go-native CV**: OpenCV and SAM 2 have mature Python ecosystems. Running as a sidecar keeps the Go monolith clean and allows independent scaling.
- **SAM 2 passthrough fallback**: Model weights are large (~2GB). Passthrough mode allows dev/CI to run without them while still exercising the full pipeline.
- **Graceful degradation**: CV service failure falls back to raw images; model router unavailable marks INCONCLUSIVE. The pipeline never hard-fails.
- **TierFull for comparison**: Structural damage comparison requires vision reasoning, not fast text processing.
