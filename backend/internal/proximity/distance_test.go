package proximity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHaversine_KnownDistance(t *testing.T) {
	// Los Angeles City Hall ↔ Santa Monica Pier: ~24.3 km
	laCityHallLat, laCityHallLng := 34.0537, -118.2428
	santaMonicaPierLat, santaMonicaPierLng := 34.0095, -118.4981

	dist := Haversine(laCityHallLat, laCityHallLng, santaMonicaPierLat, santaMonicaPierLng)

	// Allow 2% tolerance on the ground-truth distance of ~24,300 m.
	assert.InDelta(t, 24300.0, dist, 500.0)
}

func TestHaversine_SamePoint(t *testing.T) {
	dist := Haversine(34.0, -118.0, 34.0, -118.0)
	assert.InDelta(t, 0.0, dist, 0.001)
}

func TestHaversine_NorthSouth(t *testing.T) {
	// 1 degree of latitude ≈ 111,195 m at any longitude.
	dist := Haversine(0.0, 0.0, 1.0, 0.0)
	assert.InDelta(t, 111_195.0, dist, 200.0)
}

func TestIsWithinThreshold_Within(t *testing.T) {
	assert.True(t, IsWithinThreshold(99.0, 100.0))
	assert.True(t, IsWithinThreshold(100.0, 100.0)) // boundary inclusive
}

func TestIsWithinThreshold_Outside(t *testing.T) {
	assert.False(t, IsWithinThreshold(101.0, 100.0))
}
