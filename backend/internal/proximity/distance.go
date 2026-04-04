package proximity

import "math"

const earthRadiusMeters = 6_371_000.0

// Haversine returns the great-circle distance in meters between two WGS-84
// coordinate pairs.
func Haversine(lat1, lng1, lat2, lng2 float64) float64 {
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*
			math.Sin(dLng/2)*math.Sin(dLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMeters * c
}

// IsWithinThreshold returns true when distMeters ≤ thresholdMeters.
func IsWithinThreshold(distMeters, thresholdMeters float64) bool {
	return distMeters <= thresholdMeters
}

func toRad(deg float64) float64 {
	return deg * math.Pi / 180
}
