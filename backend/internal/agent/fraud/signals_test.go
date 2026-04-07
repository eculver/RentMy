package fraud

import (
	"testing"
	"time"
)

// buildBundle constructs a SignalBundle from raw signals the same way
// RunAllSignals does, applying the WiFi compound-only rule.
func buildBundle(userID string, signals []FraudSignal) SignalBundle {
	bundle := SignalBundle{UserID: userID, Signals: signals}
	for _, sig := range signals {
		if !sig.IsCompoundOnly {
			bundle.HasNonCompoundSignal = true
			bundle.CompoundScore += sig.Score
		} else {
			// Tentatively add; TotalScore() gates it.
			bundle.CompoundScore += sig.Score
		}
	}
	return bundle
}

func wifiSignal(userID string) FraudSignal {
	return FraudSignal{
		Type:           SignalWiFiNetwork,
		UserID:         userID,
		Score:          30,
		IsCompoundOnly: true,
		DetectedAt:     time.Now(),
	}
}

func deviceSignal(userID string) FraudSignal {
	return FraudSignal{
		Type:       SignalSharedDeviceFingerprint,
		UserID:     userID,
		Score:      40,
		DetectedAt: time.Now(),
	}
}

func TestSignalBundle_WiFiAlone_ScoreIsZero(t *testing.T) {
	bundle := buildBundle("user1", []FraudSignal{wifiSignal("user1")})
	if got := bundle.TotalScore(); got != 0 {
		t.Errorf("WiFi alone: TotalScore() = %d, want 0", got)
	}
}

func TestSignalBundle_WiFiPlusDevice_ScoreIncludes(t *testing.T) {
	signals := []FraudSignal{deviceSignal("user1"), wifiSignal("user1")}
	bundle := buildBundle("user1", signals)
	// 40 (device) + 30 (wifi — compound unlocked) = 70
	if got := bundle.TotalScore(); got != 70 {
		t.Errorf("WiFi + device: TotalScore() = %d, want 70", got)
	}
}

func TestSignalBundle_WiFiPlusWifi_ScoreIsZero(t *testing.T) {
	// Two WiFi signals (both compound-only) should still produce 0.
	signals := []FraudSignal{wifiSignal("user1"), wifiSignal("user1")}
	bundle := buildBundle("user1", signals)
	if got := bundle.TotalScore(); got != 0 {
		t.Errorf("WiFi + WiFi: TotalScore() = %d, want 0", got)
	}
}

func TestSignalBundle_DeviceOnly_Score(t *testing.T) {
	bundle := buildBundle("user1", []FraudSignal{deviceSignal("user1")})
	if got := bundle.TotalScore(); got != 40 {
		t.Errorf("device only: TotalScore() = %d, want 40", got)
	}
}

func TestActionFromScore(t *testing.T) {
	cases := []struct {
		score  int
		action Action
	}{
		{0, ActionMonitor},
		{79, ActionMonitor},
		{80, ActionFlag},
		{99, ActionFlag},
		{100, ActionSuspend},
		{150, ActionSuspend},
	}
	for _, tc := range cases {
		got := actionFromScore(tc.score)
		if got != tc.action {
			t.Errorf("actionFromScore(%d) = %q, want %q", tc.score, got, tc.action)
		}
	}
}

func TestIsSequentialPhone(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"4155551000", "4155551005", true},
		{"4155551000", "4155551010", true},
		{"4155551000", "4155551011", false}, // diff > 10
		{"4155551000", "4165551001", false}, // different prefix
		{"415555100", "4155551001", false},  // different length
	}
	for _, tc := range cases {
		got := isSequentialPhone(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("isSequentialPhone(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
