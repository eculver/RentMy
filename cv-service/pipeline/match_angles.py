"""Angle matching — pairs check-in and check-out photos by gyroscope orientation."""

from __future__ import annotations

import math


def _euler_distance(a: dict, b: dict) -> float:
    """Compute Euler angle distance between two orientation dicts.

    Each dict should have 'roll', 'pitch', 'yaw' keys (in degrees).
    """
    dr = a.get("roll", 0.0) - b.get("roll", 0.0)
    dp = a.get("pitch", 0.0) - b.get("pitch", 0.0)
    dy = a.get("yaw", 0.0) - b.get("yaw", 0.0)
    return math.sqrt(dr * dr + dp * dp + dy * dy)


def match_pairs(
    checkin_orientations: list[dict],
    checkout_orientations: list[dict],
) -> list[tuple[int, int]]:
    """Match check-in and check-out photos by closest orientation.

    Uses a greedy nearest-neighbor approach: for each check-out photo, find
    the closest unmatched check-in photo by Euler angle distance.

    Returns a list of (checkin_index, checkout_index) tuples.
    """
    if not checkin_orientations or not checkout_orientations:
        return []

    # Extract indices from the orientation dicts.
    ci_indices = list(range(len(checkin_orientations)))
    co_indices = list(range(len(checkout_orientations)))

    pairs: list[tuple[int, int]] = []
    used_ci: set[int] = set()

    for co_idx in co_indices:
        best_ci = -1
        best_dist = float("inf")
        for ci_idx in ci_indices:
            if ci_idx in used_ci:
                continue
            dist = _euler_distance(
                checkin_orientations[ci_idx],
                checkout_orientations[co_idx],
            )
            if dist < best_dist:
                best_dist = dist
                best_ci = ci_idx
        if best_ci >= 0:
            pairs.append((best_ci, co_idx))
            used_ci.add(best_ci)

    return pairs
