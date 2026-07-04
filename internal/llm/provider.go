package llm

import "context"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateRequest struct {
	SystemPrompt string            `json:"system_prompt"`
	Messages     []Message         `json:"messages"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type GenerateResponse struct {
	Text  string `json:"text"`
	Model string `json:"model"`
}

type Provider interface {
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
}

type FakeProvider struct{}

func NewFakeProvider() *FakeProvider {
	return &FakeProvider{}
}

func (p *FakeProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	return &GenerateResponse{
		Text:  "FakeLLM: deterministic response for tests and local demos.",
		Model: "fake-llm",
	}, nil
}

type Config struct {
	Provider       string
	APIKey         string
	BaseURL        string
	Model          string
	MaxTokens      int
	EnableThinking *bool
	Effort         string
}

func maxInt(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
