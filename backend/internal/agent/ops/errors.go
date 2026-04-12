package ops

import "errors"

var (
	// ErrNoSnapshot is returned when the snapshots table is empty.
	ErrNoSnapshot = errors.New("ops: no health snapshot found")
	// ErrRuleNotFound is returned when an alert rule ID does not exist.
	ErrRuleNotFound = errors.New("ops: alert rule not found")
	// ErrAlertNotFound is returned when an alert ID does not exist or is already acknowledged.
	ErrAlertNotFound = errors.New("ops: alert not found")
)
