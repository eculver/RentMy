package outcome

import (
	"math"
	"testing"
	"time"
)

func TestBuildCalibrationBucket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		low     float64
		high    float64
		total   int
		correct int
		wantExp float64
		wantAct float64
		wantErr float64
	}{
		{
			name:    "perfect calibration at 0.7-0.8",
			low:     0.7,
			high:    0.8,
			total:   100,
			correct: 75,
			wantExp: 0.75,
			wantAct: 0.75,
			wantErr: 0.0,
		},
		{
			name:    "overconfident at 0.9-1.0",
			low:     0.9,
			high:    1.0,
			total:   100,
			correct: 70,
			wantExp: 0.95,
			wantAct: 0.70,
			wantErr: 0.25,
		},
		{
			name:    "underconfident at 0.5-0.6",
			low:     0.5,
			high:    0.6,
			total:   100,
			correct: 80,
			wantExp: 0.55,
			wantAct: 0.80,
			wantErr: 0.25,
		},
		{
			name:    "zero decisions",
			low:     0.8,
			high:    0.9,
			total:   0,
			correct: 0,
			wantExp: 0.85,
			wantAct: 0.0,
			wantErr: 0.85,
		},
		{
			name:    "all correct at 0.6-0.7",
			low:     0.6,
			high:    0.7,
			total:   50,
			correct: 50,
			wantExp: 0.65,
			wantAct: 1.0,
			wantErr: 0.35,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := buildCalibrationBucket("TEST", tt.low, tt.high, tt.total, tt.correct)

			if b.AgentType != "TEST" {
				t.Errorf("AgentType = %q, want TEST", b.AgentType)
			}
			if b.BucketLow != tt.low {
				t.Errorf("BucketLow = %v, want %v", b.BucketLow, tt.low)
			}
			if b.BucketHigh != tt.high {
				t.Errorf("BucketHigh = %v, want %v", b.BucketHigh, tt.high)
			}
			if math.Abs(b.ExpectedAccuracy-tt.wantExp) > 0.001 {
				t.Errorf("ExpectedAccuracy = %v, want %v", b.ExpectedAccuracy, tt.wantExp)
			}
			if math.Abs(b.ActualAccuracy-tt.wantAct) > 0.001 {
				t.Errorf("ActualAccuracy = %v, want %v", b.ActualAccuracy, tt.wantAct)
			}
			if math.Abs(b.CalibrationError-tt.wantErr) > 0.001 {
				t.Errorf("CalibrationError = %v, want %v", b.CalibrationError, tt.wantErr)
			}
			if b.TotalDecisions != tt.total {
				t.Errorf("TotalDecisions = %d, want %d", b.TotalDecisions, tt.total)
			}
			if b.CorrectDecisions != tt.correct {
				t.Errorf("CorrectDecisions = %d, want %d", b.CorrectDecisions, tt.correct)
			}
			if b.UpdatedAt.IsZero() {
				t.Error("UpdatedAt should not be zero")
			}
		})
	}
}

func TestCalibrationBuckets(t *testing.T) {
	t.Parallel()

	if len(CalibrationBuckets) != 5 {
		t.Fatalf("expected 5 calibration buckets, got %d", len(CalibrationBuckets))
	}

	expected := []struct{ low, high float64 }{
		{0.5, 0.6},
		{0.6, 0.7},
		{0.7, 0.8},
		{0.8, 0.9},
		{0.9, 1.0},
	}

	for i, b := range CalibrationBuckets {
		if b.Low != expected[i].low || b.High != expected[i].high {
			t.Errorf("bucket %d: got [%.1f, %.1f), want [%.1f, %.1f)",
				i, b.Low, b.High, expected[i].low, expected[i].high)
		}
	}
}

func TestCalibrationReport_Aggregation(t *testing.T) {
	t.Parallel()

	buckets := []CalibrationBucket{
		{AgentType: "DISPUTE", BucketLow: 0.5, BucketHigh: 0.6, TotalDecisions: 20, CorrectDecisions: 11, CalibrationError: 0.0, UpdatedAt: time.Now()},
		{AgentType: "DISPUTE", BucketLow: 0.6, BucketHigh: 0.7, TotalDecisions: 30, CorrectDecisions: 20, CalibrationError: 0.02, UpdatedAt: time.Now()},
		{AgentType: "DISPUTE", BucketLow: 0.7, BucketHigh: 0.8, TotalDecisions: 50, CorrectDecisions: 38, CalibrationError: 0.01, UpdatedAt: time.Now()},
	}

	report := CalibrationReport{
		AgentType: "DISPUTE",
		Buckets:   buckets,
	}

	var totalCalErr float64
	var correctDecisions int
	for _, b := range buckets {
		report.TotalDecisions += b.TotalDecisions
		correctDecisions += b.CorrectDecisions
		totalCalErr += b.CalibrationError
	}
	if report.TotalDecisions > 0 {
		report.OverallAccuracy = float64(correctDecisions) / float64(report.TotalDecisions)
	}
	report.MeanCalibration = totalCalErr / float64(len(buckets))

	if report.TotalDecisions != 100 {
		t.Errorf("TotalDecisions = %d, want 100", report.TotalDecisions)
	}
	if math.Abs(report.OverallAccuracy-0.69) > 0.01 {
		t.Errorf("OverallAccuracy = %v, want ~0.69", report.OverallAccuracy)
	}
	if math.Abs(report.MeanCalibration-0.01) > 0.001 {
		t.Errorf("MeanCalibration = %v, want ~0.01", report.MeanCalibration)
	}
}

func TestOutcomeLinkJobArgs_Kind(t *testing.T) {
	args := OutcomeLinkJobArgs{TransactionID: "test-123"}
	if args.Kind() != "outcome_link" {
		t.Errorf("Kind() = %q, want %q", args.Kind(), "outcome_link")
	}
}

func TestMonthlyCalibrationReportJobArgs_Kind(t *testing.T) {
	args := MonthlyCalibrationReportJobArgs{}
	if args.Kind() != "monthly_calibration_report" {
		t.Errorf("Kind() = %q, want %q", args.Kind(), "monthly_calibration_report")
	}
}

func TestDecisionWithOutcome_Fields(t *testing.T) {
	t.Parallel()

	correct := true
	txnID := "txn-123"
	conf := 0.85
	d := DecisionWithOutcome{
		ID:             "dec-1",
		AgentType:      "DISPUTE",
		TransactionID:  &txnID,
		Confidence:     &conf,
		Escalated:      false,
		OutcomeCorrect: &correct,
		OutcomeID:      nil,
		CreatedAt:      time.Now(),
	}

	if d.ID != "dec-1" {
		t.Errorf("ID = %q, want dec-1", d.ID)
	}
	if *d.OutcomeCorrect != true {
		t.Error("OutcomeCorrect should be true")
	}
}
