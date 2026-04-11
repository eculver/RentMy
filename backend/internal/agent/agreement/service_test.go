package agreement

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// repoPromptsDir resolves the real prompts/ directory relative to this test file.
func repoPromptsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	// file is …/backend/internal/agent/agreement/service_test.go
	// walk up 3 levels to backend/, then into prompts/
	base := filepath.Join(filepath.Dir(file), "..", "..", "..", "prompts")
	abs, err := filepath.Abs(base)
	require.NoError(t, err)
	return abs
}

// newTestService creates a Service with no router (deterministic only) and
// the real prompts directory injected so loadBaseTemplate works in CI.
func newTestService(t *testing.T) *Service {
	t.Helper()
	svc := &Service{
		promptsDir: repoPromptsDir(t),
	}
	return svc
}

// TestLoadBaseTemplate verifies the base template parses and contains required sections.
func TestLoadBaseTemplate(t *testing.T) {
	svc := newTestService(t)
	tmpl, version, err := svc.loadBaseTemplate()
	require.NoError(t, err)
	require.NotEmpty(t, version)
	require.NotEmpty(t, tmpl.Sections)

	// Immutable sections must be present.
	required := []string{"parties", "liability", "governing_law", "hold_and_damage"}
	sectionIDs := make(map[string]bool)
	for _, s := range tmpl.Sections {
		sectionIDs[s.ID] = true
	}
	for _, id := range required {
		require.True(t, sectionIDs[id], "missing required section: %s", id)
	}

	// item_specific_clauses must be dynamic.
	var hasDynamic bool
	for _, s := range tmpl.Sections {
		if s.Dynamic {
			hasDynamic = true
			require.Equal(t, "agreement_agent", s.Source)
		}
	}
	require.True(t, hasDynamic, "expected at least one dynamic section")
}

// TestMergeClauses verifies clauses are injected into the dynamic section.
func TestMergeClauses(t *testing.T) {
	svc := newTestService(t)
	tmpl, _, err := svc.loadBaseTemplate()
	require.NoError(t, err)

	clauses := []Clause{
		{Title: "No Water Exposure", Text: "Keep the item dry at all times.", Category: ClauseCategoryCareRequirement},
		{Title: "Approved Surfaces Only", Text: "Use on approved surfaces only.", Category: ClauseCategoryUseRestriction},
	}

	merged := mergeClauses(tmpl, clauses)

	var dynamicSection *baseSection
	for i, s := range merged.Sections {
		if s.Dynamic {
			dynamicSection = &merged.Sections[i]
		}
	}
	require.NotNil(t, dynamicSection, "dynamic section not found")
	require.Len(t, dynamicSection.Clauses, 2)
	require.Equal(t, "No Water Exposure", dynamicSection.Clauses[0].Title)
}

// TestGuardrailCheck verifies banned patterns are detected.
func TestGuardrailCheck(t *testing.T) {
	cases := []struct {
		name    string
		clause  Clause
		wantBad bool
	}{
		{
			name: "valid care clause",
			clause: Clause{
				Title:    "Keep Dry",
				Text:     "Do not submerge in water or expose to rain.",
				Category: ClauseCategoryCareRequirement,
			},
			wantBad: false,
		},
		{
			name: "valid safety clause",
			clause: Clause{
				Title:    "Helmet Required",
				Text:     "Renter must wear a helmet at all times when using this item.",
				Category: ClauseCategorySafety,
			},
			wantBad: false,
		},
		{
			name: "liability limitation attempt",
			clause: Clause{
				Title:    "Liability Limitation",
				Text:     "This clause will limit liability for damage.",
				Category: ClauseCategoryDamageDefinition,
			},
			wantBad: true,
		},
		{
			name: "arbitration waiver attempt",
			clause: Clause{
				Title:    "Dispute Resolution",
				Text:     "Renter waives arbitration rights for claims under $500.",
				Category: ClauseCategoryUseRestriction,
			},
			wantBad: true,
		},
		{
			name: "fee waiver attempt",
			clause: Clause{
				Title:    "No Late Fee",
				Text:     "Host agrees to waive fee for late returns.",
				Category: ClauseCategoryUseRestriction,
			},
			wantBad: true,
		},
		{
			name: "valid damage definition",
			clause: Clause{
				Title:    "Scratch Threshold",
				Text:     "Scratches deeper than 2mm on the lens surface constitute damage.",
				Category: ClauseCategoryDamageDefinition,
			},
			wantBad: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason := guardrailCheck(tc.clause)
			if tc.wantBad {
				require.NotEmpty(t, reason, "expected guardrail to flag clause")
			} else {
				require.Empty(t, reason, "expected guardrail to pass clause, got: %s", reason)
			}
		})
	}
}

// TestGuardrails verifies that violating clauses are stripped and safe ones are kept.
func TestGuardrails(t *testing.T) {
	svc := newTestService(t)
	svc.modelRouter = nil // deterministic only

	input := []Clause{
		{Title: "Limit Liability", Text: "This limits all liability for damages.", Category: ClauseCategoryDamageDefinition},
		{Title: "Keep Dry", Text: "Do not submerge in water.", Category: ClauseCategoryCareRequirement},
		{Title: "No Arbitration", Text: "Renter agrees not to subject disputes to arbitration.", Category: ClauseCategoryUseRestriction},
		{Title: "Helmet Required", Text: "Safety helmet must be worn at all times.", Category: ClauseCategorySafety},
	}

	var valid []Clause
	var warnings []string
	for _, c := range input {
		if reason := guardrailCheck(c); reason != "" {
			warnings = append(warnings, reason)
		} else {
			valid = append(valid, c)
		}
	}

	require.Len(t, valid, 2, "expected 2 safe clauses")
	require.Len(t, warnings, 2, "expected 2 guardrail warnings")

	validTitles := []string{valid[0].Title, valid[1].Title}
	require.Contains(t, validTitles, "Keep Dry")
	require.Contains(t, validTitles, "Helmet Required")
}

// TestMergedAgreementIsValidJSON verifies the final merged agreement serialises cleanly.
func TestMergedAgreementIsValidJSON(t *testing.T) {
	svc := newTestService(t)
	tmpl, _, err := svc.loadBaseTemplate()
	require.NoError(t, err)

	clauses := []Clause{
		{Title: "Power-off Required", Text: "Return the item powered off and fully charged.", Category: ClauseCategoryCareRequirement},
	}
	merged := mergeClauses(tmpl, clauses)

	data, err := json.Marshal(merged)
	require.NoError(t, err)
	require.True(t, json.Valid(data), "merged agreement is not valid JSON")

	// Round-trip parse.
	var rt baseTemplate
	require.NoError(t, json.Unmarshal(data, &rt))
	require.Equal(t, tmpl.Version, rt.Version)
}
