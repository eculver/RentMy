package router

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const (
	defaultMaxTokensCheap = 1024
	defaultMaxTokensFull  = 4096
	defaultTimeoutCheap   = 30 * time.Second
	defaultTimeoutFull    = 120 * time.Second
	maxRetries            = 3
)

// Router dispatches agent tasks to the appropriate model tier.
type Router interface {
	Route(ctx context.Context, input RouteInput, opts ...Option) (RouteOutput, error)
}

// Option configures a single Route call.
type Option func(*routeConfig)

type routeConfig struct {
	maxRetries  int
	timeout     time.Duration
	forceModel  string // override model string (testing)
}

// WithMaxRetries sets the maximum number of API call retries.
func WithMaxRetries(n int) Option {
	return func(c *routeConfig) { c.maxRetries = n }
}

// WithTimeout sets the per-call timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *routeConfig) { c.timeout = d }
}

// AnthropicRouter is the default Router implementation backed by the Anthropic API.
type AnthropicRouter struct {
	client      anthropic.Client
	cheapModel  string // e.g. "claude-haiku-4-5"
	fullModel   string // e.g. "claude-sonnet-4-6"
	promptCache *promptCache
}

// Config holds the configuration for creating an AnthropicRouter.
type Config struct {
	APIKey     string
	CheapModel string // defaults to claude-haiku-4-5
	FullModel  string // defaults to claude-sonnet-4-6
	PromptsDir string // path to the prompts directory
}

// New creates an AnthropicRouter from the provided Config.
func New(cfg Config) (*AnthropicRouter, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("router: anthropic API key is required")
	}
	cheapModel := cfg.CheapModel
	if cheapModel == "" {
		cheapModel = string(anthropic.ModelClaudeHaiku4_5)
	}
	fullModel := cfg.FullModel
	if fullModel == "" {
		fullModel = string(anthropic.ModelClaudeSonnet4_6)
	}
	promptsDir := cfg.PromptsDir
	if promptsDir == "" {
		promptsDir = "prompts"
	}

	client := anthropic.NewClient(option.WithAPIKey(cfg.APIKey))

	return &AnthropicRouter{
		client:      client,
		cheapModel:  cheapModel,
		fullModel:   fullModel,
		promptCache: newPromptCache(promptsDir),
	}, nil
}

// RenderPrompt renders the latest prompt template for the given agent name with data.
// Returns the rendered text and the version string (e.g., "v1").
func (r *AnthropicRouter) RenderPrompt(agentName string, data any) (string, string, error) {
	return r.promptCache.Render(agentName, data)
}

// Route dispatches the input to the appropriate model tier based on the task.
// TierNone tasks return immediately with an empty RouteOutput.
func (r *AnthropicRouter) Route(ctx context.Context, input RouteInput, opts ...Option) (RouteOutput, error) {
	cfg := &routeConfig{
		maxRetries: maxRetries,
	}
	for _, o := range opts {
		o(cfg)
	}

	tier, err := TierFor(input.Task)
	if err != nil {
		return RouteOutput{}, err
	}

	if tier == TierNone {
		return RouteOutput{}, nil
	}

	modelID, timeout := r.modelForTier(tier)
	if cfg.timeout > 0 {
		timeout = cfg.timeout
	}
	if cfg.forceModel != "" {
		modelID = cfg.forceModel
	}

	maxTok := input.MaxTokens
	if maxTok <= 0 {
		if tier == TierFull {
			maxTok = defaultMaxTokensFull
		} else {
			maxTok = defaultMaxTokensCheap
		}
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	msg, err := r.callWithRetry(callCtx, modelID, input, maxTok, cfg.maxRetries)
	latency := time.Since(start)
	if err != nil {
		return RouteOutput{}, fmt.Errorf("router: calling %s for task %s: %w", modelID, input.Task, err)
	}

	content := ""
	if len(msg.Content) > 0 {
		content = msg.Content[0].AsText().Text
	}

	cached := msg.Usage.CacheReadInputTokens > 0

	return RouteOutput{
		Content:      content,
		Model:        string(msg.Model),
		InputTokens:  int(msg.Usage.InputTokens + msg.Usage.CacheReadInputTokens),
		OutputTokens: int(msg.Usage.OutputTokens),
		Latency:      latency,
		Cached:       cached,
	}, nil
}

func (r *AnthropicRouter) modelForTier(tier ModelTier) (string, time.Duration) {
	if tier == TierFull {
		return r.fullModel, defaultTimeoutFull
	}
	return r.cheapModel, defaultTimeoutCheap
}

func (r *AnthropicRouter) callWithRetry(ctx context.Context, modelID string, input RouteInput, maxTokens int, retries int) (*anthropic.Message, error) {
	var lastErr error
	backoff := 500 * time.Millisecond

	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			// Exponential backoff with a cap at 8 seconds.
			backoff *= 2
			if backoff > 8*time.Second {
				backoff = 8 * time.Second
			}
		}

		params, err := r.buildParams(modelID, input, maxTokens)
		if err != nil {
			return nil, err
		}

		msg, err := r.client.Messages.New(ctx, params)
		if err == nil {
			return msg, nil
		}
		lastErr = err
		slog.Warn("router: API call failed, retrying", "attempt", attempt+1, "max", retries, "error", err)
	}
	return nil, lastErr
}

func (r *AnthropicRouter) buildParams(modelID string, input RouteInput, maxTokens int) (anthropic.MessageNewParams, error) {
	var contentBlocks []anthropic.ContentBlockParamUnion

	// Add images first (for vision tasks), then the text prompt.
	for _, img := range input.Images {
		encoded := base64.StdEncoding.EncodeToString(img.Data)
		contentBlocks = append(contentBlocks, anthropic.NewImageBlockBase64(img.MediaType, encoded))
	}
	contentBlocks = append(contentBlocks, anthropic.NewTextBlock(input.UserPrompt))

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(modelID),
		MaxTokens: int64(maxTokens),
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(contentBlocks...)},
	}

	if input.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: input.SystemPrompt}}
	}

	return params, nil
}
