package llm

import (
	"context"
	"fmt"
	"strings"
)

type OpenAICompatibleProvider struct {
	http      *HTTPClient
	apiKey    string
	baseURL   string
	model     string
	maxTokens int
}

func NewOpenAICompatibleProvider(cfg Config) (*OpenAICompatibleProvider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, ErrMissingAPIKey
	}
	baseURL := strings.TrimRight(firstText(cfg.BaseURL, "https://api.openai.com/v1"), "/")
	return &OpenAICompatibleProvider{
		http:      NewHTTPClient(),
		apiKey:    cfg.APIKey,
		baseURL:   baseURL,
		model:     firstText(cfg.Model, "gpt-4o-mini"),
		maxTokens: maxInt(cfg.MaxTokens, 4096),
	}, nil
}

func (p *OpenAICompatibleProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	messages := make([]map[string]string, 0, len(req.Messages)+1)
	if strings.TrimSpace(req.SystemPrompt) != "" {
		messages = append(messages, map[string]string{"role": "system", "content": req.SystemPrompt})
	}
	for _, msg := range req.Messages {
		role := msg.Role
		if role != "assistant" && role != "system" {
			role = "user"
		}
		messages = append(messages, map[string]string{"role": role, "content": msg.Content})
	}

	body := map[string]any{
		"model":      p.model,
		"messages":   messages,
		"max_tokens": p.maxTokens,
	}
	var response struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := p.http.postJSON(ctx, p.baseURL+"/chat/completions", map[string]string{"Authorization": "Bearer " + p.apiKey}, body, &response); err != nil {
		return nil, err
	}
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("openai-compatible provider returned no choices")
	}
	return &GenerateResponse{Text: response.Choices[0].Message.Content, Model: firstText(response.Model, p.model)}, nil
}
