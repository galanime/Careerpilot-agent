package llm

import (
	"context"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type AnthropicProvider struct {
	client          anthropic.Client
	model           anthropic.Model
	maxTokens       int
	thinkingEnabled bool
	effort          string
}

func NewAnthropicProvider(cfg Config) (*AnthropicProvider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, ErrMissingAPIKey
	}
	thinkingEnabled := true
	if cfg.EnableThinking != nil {
		thinkingEnabled = *cfg.EnableThinking
	}
	return &AnthropicProvider{
		client:          anthropic.NewClient(option.WithAPIKey(cfg.APIKey)),
		model:           anthropic.Model(firstText(cfg.Model, "claude-opus-4-7")),
		maxTokens:       maxInt(cfg.MaxTokens, 16000),
		thinkingEnabled: thinkingEnabled,
		effort:          firstText(cfg.Effort, "high"),
	}, nil
}

func (p *AnthropicProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	userPrompt := messagesToPrompt(GenerateRequest{Messages: req.Messages})
	if strings.TrimSpace(userPrompt) == "" {
		userPrompt = "Generate a concise response."
	}

	params := anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: int64(p.maxTokens),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	}
	if strings.TrimSpace(req.SystemPrompt) != "" {
		params.System = []anthropic.TextBlockParam{{
			Text:         req.SystemPrompt,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}}
	}
	if p.thinkingEnabled {
		adaptive := anthropic.ThinkingConfigAdaptiveParam{}
		params.Thinking = anthropic.ThinkingConfigParamUnion{OfAdaptive: &adaptive}
		params.OutputConfig = anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffort(p.effort)}
	}

	response, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, err
	}

	var builder strings.Builder
	for _, block := range response.Content {
		switch value := block.AsAny().(type) {
		case anthropic.TextBlock:
			builder.WriteString(value.Text)
		}
	}
	return &GenerateResponse{Text: builder.String(), Model: string(response.Model)}, nil
}
