package booking

import (
	"testing"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		from Status
		to   Status
		want bool
	}{
		// Valid transitions
		{StatusRequested, StatusAccepted, true},
		{StatusRequested, StatusDeclined, true},
		{StatusRequested, StatusAutoDeclined, true},
		{StatusRequested, StatusCancelled, true},
		{StatusAccepted, StatusActive, true},
		{StatusAccepted, StatusCancelled, true},
		{StatusActive, StatusCompleted, true},
		{StatusActive, StatusDisputed, true},
		{StatusDisputed, StatusCompleted, true},

		// Invalid transitions
		{StatusRequested, StatusActive, false},
		{StatusRequested, StatusCompleted, false},
		{StatusRequested, StatusDisputed, false},
		{StatusAccepted, StatusRequested, false},
		{StatusAccepted, StatusDeclined, false},
		{StatusAccepted, StatusAutoDeclined, false},
		{StatusActive, StatusRequested, false},
		{StatusActive, StatusAccepted, false},
		{StatusActive, StatusCancelled, false},

		// Terminal states have no outgoing transitions
		{StatusCompleted, StatusDisputed, false},
		{StatusCompleted, StatusActive, false},
		{StatusDeclined, StatusRequested, false},
		{StatusAutoDeclined, StatusRequested, false},
		{StatusCancelled, StatusRequested, false},
		{StatusCancelled, StatusAccepted, false},
	}

	for _, tt := range tests {
		got := CanTransition(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("CanTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestValidateTransition_Valid(t *testing.T) {
	if err := ValidateTransition(StatusRequested, StatusAccepted); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateTransition_Invalid(t *testing.T) {
	err := ValidateTransition(StatusCompleted, StatusActive)
	if err == nil {
		t.Fatal("expected error for invalid transition, got nil")
	}
}
