package photodiff

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLLMResponse_Valid(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   LLMComparisonResponse
	}{
		{
			name:  "no change",
			input: `{"classification":"NO_CHANGE","confidence":0.95,"details":"Item appears identical."}`,
			want: LLMComparisonResponse{
				Classification: "NO_CHANGE",
				Confidence:     0.95,
				Details:        "Item appears identical.",
			},
		},
		{
			name:  "functional damage",
			input: `{"classification":"FUNCTIONAL_DAMAGE","confidence":0.87,"details":"Screen has a visible crack."}`,
			want: LLMComparisonResponse{
				Classification: "FUNCTIONAL_DAMAGE",
				Confidence:     0.87,
				Details:        "Screen has a visible crack.",
			},
		},
		{
			name: "with markdown fences",
			input: "```json\n{\"classification\":\"COSMETIC_DAMAGE\",\"confidence\":0.78,\"details\":\"Minor scratch on back.\"}\n```",
			want: LLMComparisonResponse{
				Classification: "COSMETIC_DAMAGE",
				Confidence:     0.78,
				Details:        "Minor scratch on back.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLLMResponse(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want.Classification, got.Classification)
			assert.InDelta(t, tt.want.Confidence, got.Confidence, 0.001)
			assert.Equal(t, tt.want.Details, got.Details)
		})
	}
}

func TestParseLLMResponse_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "not json", input: "this is not json"},
		{name: "empty classification", input: `{"classification":"","confidence":0.5,"details":"test"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseLLMResponse(tt.input)
			assert.Error(t, err)
		})
	}
}

func TestParseS3URL(t *testing.T) {
	tests := []struct {
		url        string
		wantBucket string
		wantKey    string
	}{
		{
			url:        "http://localhost:9002/media-originals/abc123",
			wantBucket: "media-originals",
			wantKey:    "abc123",
		},
		{
			url:        "https://s3.amazonaws.com/media-originals/path/to/file",
			wantBucket: "media-originals",
			wantKey:    "path/to/file",
		},
		{
			url:        "invalid",
			wantBucket: "",
			wantKey:    "",
		},
		{
			url:        "http://localhost:9002/",
			wantBucket: "",
			wantKey:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			bucket, key := parseS3URL(tt.url)
			assert.Equal(t, tt.wantBucket, bucket)
			assert.Equal(t, tt.wantKey, key)
		})
	}
}

func TestValidResults(t *testing.T) {
	assert.True(t, ValidResults[ResultNoChange])
	assert.True(t, ValidResults[ResultCosmeticDamage])
	assert.True(t, ValidResults[ResultFunctionalDamage])
	assert.True(t, ValidResults[ResultMissingItem])
	assert.True(t, ValidResults[ResultInconclusive])
	assert.False(t, ValidResults[DiffResult("INVALID")])
}
