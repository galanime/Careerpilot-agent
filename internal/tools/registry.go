package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

func (r *Registry) MustGet(name string) Tool {
	tool, ok := r.tools[name]
	if !ok {
		panic(fmt.Sprintf("tool %q is not registered", name))
	}
	return tool
}

func (r *Registry) Execute(ctx context.Context, name string, input any) (json.RawMessage, error) {
	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool %q is not registered", name)
	}

	payload, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	return tool.Execute(ctx, payload)
}

func Decode[T any](input json.RawMessage) (T, error) {
	var value T
	err := json.Unmarshal(input, &value)
	return value, err
}

func Encode(value any) (json.RawMessage, error) {
	return json.Marshal(value)
}
