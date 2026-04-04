#!/usr/bin/env bash
# osrm-prepare.sh — Download a regional OSM extract and process it for OSRM.
#
# Usage:
#   docker compose run --rm osrm bash scripts/osrm-prepare.sh
#
# The processed files are written to the osrm_data Docker volume (/data).
# After this script completes, start the OSRM service with:
#   docker compose --profile osrm up -d osrm
#
# To use a different region, change the OSRM_EXTRACT_URL below.
# All available extracts: https://download.geofabrik.de/

set -euo pipefail

DATA_DIR="/data"
REGION_PBF="${DATA_DIR}/region.osm.pbf"
REGION_OSRM="${DATA_DIR}/region.osrm"

# Default: California. Override with OSRM_EXTRACT_URL env var.
EXTRACT_URL="${OSRM_EXTRACT_URL:-https://download.geofabrik.de/north-america/us/california-latest.osm.pbf}"

echo "Downloading OSM extract from ${EXTRACT_URL} ..."
wget -O "${REGION_PBF}" "${EXTRACT_URL}"

echo "Extracting routing graph ..."
osrm-extract -p /opt/car.lua "${REGION_PBF}"

echo "Partitioning ..."
osrm-partition "${REGION_OSRM}"

echo "Customizing ..."
osrm-customize "${REGION_OSRM}"

echo "Done. Start OSRM with: docker compose --profile osrm up -d osrm"
