package llm

import (
	"context"
	"fmt"
	"strings"
)

type OllamaProvider struct {
	http    *HTTPClient
	baseURL string
	model   string
}

func NewOllamaProvider(cfg Config) *OllamaProvider {
	return &OllamaProvider{
		http:    NewHTTPClient(),
		baseURL: strings.TrimRight(firstText(cfg.BaseURL, "http://127.0.0.1:11434"), "/"),
		model:   firstText(cfg.Model, "llama3.1"),
	}
}

func (p *OllamaProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	body := map[string]any{
		"model":  p.model,
		"prompt": messagesToPrompt(req),
		"stream": false,
	}
	var response struct {
		Model    string `json:"model"`
		Response string `json:"response"`
	}
	if err := p.http.postJSON(ctx, p.baseURL+"/api/generate", nil, body, &response); err != nil {
		return nil, err
	}
	if strings.TrimSpace(response.Response) == "" {
		return nil, fmt.Errorf("ollama provider returned empty response")
	}
	return &GenerateResponse{Text: response.Response, Model: firstText(response.Model, p.model)}, nil
}
