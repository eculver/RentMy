# Task 6.1 — Photo Diff Pipeline

## Summary

Implemented the two-stage photo diff pipeline for comparing check-in and check-out rental photos to detect damage. Stage 1 uses a Python CV sidecar for preprocessing (normalization, segmentation, angle matching). Stage 2 uses the LLM model router for structural comparison and damage classification.

## What was built

### Python CV Sidecar (`cv-service/`)
- **FastAPI application** with three endpoints: `/health`, `/preprocess`, `/quality`
- **Normalization pipeline** (`pipeline/normalize.py`): resize to 1024px longest side, CLAHE histogram equalization on LAB color space, gray-world white balance correction
- **Segmentation** (`pipeline/segment.py`): SAM 2 integration with lazy loading and passthrough fallback when model weights are unavailable
- **Angle matching** (`pipeline/match_angles.py`): pairs check-in/check-out photos by closest gyroscope orientation using Euler angle distance
- **Quality checks** (`pipeline/quality.py`): Laplacian blur detection, minimum resolution enforcement
- **Dockerfile**: Python 3.11-slim with OpenCV headless, configured for port 8090

### Go CV Client (`backend/internal/platform/cv/`)
- HTTP client with configurable timeouts (30s preprocess, 5s quality)
- Multipart form upload for image data
- Health check method
- Functional options pattern

### PhotoDiff Service (`backend/internal/photodiff/`)
- **model.go**: DiffResult enum (NO_CHANGE, COSMETIC_DAMAGE, FUNCTIONAL_DAMAGE, MISSING_ITEM, INCONCLUSIVE), PhotoDiff struct, LLMComparisonResponse
- **repository.go**: UpdateDiffResult and GetDiffResult on transactions table (uses existing photo_diff_result/confidence columns)
- **service.go**: Full pipeline orchestration — fetch media, download from S3, CV preprocess, LLM comparison, parse response, store result. Graceful degradation: falls back to raw images if cv-service unavailable, marks INCONCLUSIVE if model router unavailable
- **handler.go**: POST/GET `/api/v1/transactions/:id/photo-diff` endpoints
- **prompt.go**: Prompt version tracking

### LLM Prompt (`backend/prompts/photodiff/v1.txt`)
- Versioned prompt template for structural damage comparison
- Returns JSON with classification, confidence (0-1.0), and details

### Infrastructure
- **docker-compose.yml**: Added cv-service container on port 8090 with health check and restart policy
- **config.go**: Added `CVServiceURL` env var (default `http://localhost:8090`)
- **tier_matrix.go**: Added `TaskPhotoDiffComparison` → TierFull (Sonnet for vision reasoning)
- **server.go**: Wired PhotoDiffService with all dependencies, mounted handler

## Tests
- `photodiff/service_test.go`: LLM response parsing (valid/invalid), S3 URL parsing, DiffResult validation
- `platform/cv/client_test.go`: Health check, quality check, preprocess — all using httptest servers

## Verification
- `go vet ./...` — clean
- `go build ./cmd/server` — clean
- `go test ./... -count=1` — all pass (including existing tests)
- `go test ./internal/photodiff/... -v -count=1` — pass
- `go test ./internal/platform/cv/... -v -count=1` — pass
- `go test ./tests/integration/... -v -count=1` — pass

## Design decisions
- SAM 2 segmentation runs in passthrough mode until model weights are provisioned — allows development and CI without large model files
- CV preprocessing failure is non-fatal: falls back to raw images for LLM comparison
- S3 URL parsing handles both localhost (MinIO) and AWS-style URLs
- Used `image/jpeg` media type consistently since normalization encodes to JPEG

## Dependencies added
- None for Go (uses existing pgx, chi, anthropic-sdk-go, testify)
- Python sidecar: fastapi, uvicorn, opencv-python-headless, Pillow, httpx, numpy

## Branch mode
- Graphite (`gt create task-6.1-photo-diff-pipeline`)
