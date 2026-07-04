package llm

import (
	"fmt"
	"strings"
)

func NewProvider(cfg Config) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "", "fake":
		return NewFakeProvider(), nil
	case "anthropic", "claude":
		return NewAnthropicProvider(cfg)
	case "openai", "openai-compatible", "compatible":
		return NewOpenAICompatibleProvider(cfg)
	case "ollama", "local":
		return NewOllamaProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported llm provider %q", cfg.Provider)
	}
}
