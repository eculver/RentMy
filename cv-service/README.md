# CV Service

Python FastAPI sidecar for computer vision preprocessing in the RentMy photo diff pipeline.

## Purpose

Stage 1 of the two-stage photo diff pipeline: cheap, fast CV operations (normalization, segmentation, angle matching, quality checks) that prepare images before Stage 2 LLM reasoning.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Liveness + readiness probe |
| POST | `/preprocess` | Normalize, segment, and pair check-in/out photos |
| POST | `/quality` | Quality checks on a single image (blur, resolution) |

## Running locally

```bash
docker compose up -d cv-service
curl http://localhost:8090/health
```

## SAM 2 Model

The segmentation pipeline uses SAM 2 when model weights are available at `models/sam2/`. Without the model, segmentation runs in passthrough mode (images returned as-is).

To download model weights, place them in the `models/` directory or mount via Docker volume.
