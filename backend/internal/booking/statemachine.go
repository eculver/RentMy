package booking

import "fmt"

// AllowedTransitions defines every valid booking state change.
// Any transition not in this map is rejected with ErrInvalidTransition.
var AllowedTransitions = map[Status][]Status{
	StatusRequested: {StatusAccepted, StatusDeclined, StatusAutoDeclined, StatusCancelled},
	StatusAccepted:  {StatusActive, StatusCancelled},
	StatusActive:    {StatusCompleted, StatusDisputed},
	StatusDisputed:  {StatusCompleted},
	// Terminal states: StatusCompleted, StatusDeclined, StatusAutoDeclined, StatusCancelled
}

// CanTransition reports whether a transition from → to is valid.
func CanTransition(from, to Status) bool {
	allowed, ok := AllowedTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// ValidateTransition returns an error if the from → to transition is not allowed.
func ValidateTransition(from, to Status) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, from, to)
	}
	return nil
}
