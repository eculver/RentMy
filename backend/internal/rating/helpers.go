package rating

import (
	"encoding/json"
	"strings"
)

// marshalBubbles serialises a slice of Bubble values to JSON bytes.
func marshalBubbles(bubbles []Bubble) ([]byte, error) {
	if bubbles == nil {
		bubbles = []Bubble{}
	}
	return json.Marshal(bubbles)
}

// unmarshalBubbles deserialises JSON bytes to a slice of Bubble values.
func unmarshalBubbles(data []byte) ([]Bubble, error) {
	if len(data) == 0 {
		return []Bubble{}, nil
	}
	var bubbles []Bubble
	if err := json.Unmarshal(data, &bubbles); err != nil {
		return nil, err
	}
	return bubbles, nil
}

// isUniqueViolation reports whether err is a Postgres 23505 unique-constraint violation.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key")
}
