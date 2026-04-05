package router

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTierMatrixComplete verifies that every AgentTask constant defined in
// model.go is present in tier_matrix.go.
func TestTierMatrixComplete(t *testing.T) {
	allDefined := []AgentTask{
		TaskItemIdentification,
		TaskTagGeneration,
		TaskValueOverrideReview,
		TaskEvidenceAnalysis,
		TaskEvidenceSummary,
		TaskRiskScoring,
		TaskKYCInterpretation,
		TaskCustomClauseGeneration,
		TaskTemplateRendering,
		TaskEscalationDecision,
		TaskLateFeeCalculation,
		TaskPatternDetection,
		TaskSignalAggregation,
		TaskAnomalyDetection,
		TaskHealthReport,
		TaskNotificationText,
		TaskSemanticSearch,
	}

	for _, task := range allDefined {
		_, err := TierFor(task)
		assert.NoError(t, err, "task %q is missing from tier_matrix.go", task)
	}
}

// TestTierLookupCorrectValues checks specific tasks return the expected tier.
func TestTierLookupCorrectValues(t *testing.T) {
	cases := []struct {
		task     AgentTask
		wantTier ModelTier
	}{
		{TaskItemIdentification, TierFull},
		{TaskTagGeneration, TierCheap},
		{TaskValueOverrideReview, TierFull},
		{TaskTemplateRendering, TierNone},
		{TaskLateFeeCalculation, TierNone},
		{TaskRiskScoring, TierCheap},
		{TaskEscalationDecision, TierFull},
		{TaskPatternDetection, TierFull},
		{TaskSignalAggregation, TierCheap},
	}

	for _, tc := range cases {
		got, err := TierFor(tc.task)
		require.NoError(t, err)
		assert.Equal(t, tc.wantTier, got, "task %q", tc.task)
	}
}

// TestUnknownTaskReturnsError checks that an unregistered task returns an error.
func TestUnknownTaskReturnsError(t *testing.T) {
	_, err := TierFor("nonexistent.task")
	require.Error(t, err)
	var e *UnknownTaskError
	assert.ErrorAs(t, err, &e)
}

// TestPromptLoaderLatestVersion verifies version detection and rendering.
func TestPromptLoaderLatestVersion(t *testing.T) {
	dir := t.TempDir()

	// Create versioned prompt files.
	agentDir := filepath.Join(dir, "test_agent")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "v1.txt"), []byte("hello v1 {{.Name}}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "v2.txt"), []byte("hello v2 {{.Name}}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "v10.txt"), []byte("hello v10 {{.Name}}"), 0o644))

	cache := newPromptCache(dir)

	version, err := cache.LatestVersion("test_agent")
	require.NoError(t, err)
	assert.Equal(t, "v10", version)

	rendered, ver, err := cache.Render("test_agent", map[string]string{"Name": "RentMy"})
	require.NoError(t, err)
	assert.Equal(t, "v10", ver)
	assert.Equal(t, "hello v10 RentMy", rendered)
}

// TestPromptLoaderNoVersions verifies an error when no versioned prompts exist.
func TestPromptLoaderNoVersions(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "empty_agent")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	cache := newPromptCache(dir)
	_, err := cache.LatestVersion("empty_agent")
	require.Error(t, err)
}

// TestNewRouterRequiresAPIKey ensures that New returns an error when no API key is supplied.
func TestNewRouterRequiresAPIKey(t *testing.T) {
	_, err := New(Config{APIKey: "", PromptsDir: t.TempDir()})
	require.Error(t, err)
}
