package llm

import "strings"

func messagesToPrompt(req GenerateRequest) string {
	var builder strings.Builder
	if strings.TrimSpace(req.SystemPrompt) != "" {
		builder.WriteString("System:\n")
		builder.WriteString(req.SystemPrompt)
		builder.WriteString("\n\n")
	}
	for _, msg := range req.Messages {
		builder.WriteString(roleTitle(msg.Role))
		builder.WriteString(":\n")
		builder.WriteString(msg.Content)
		builder.WriteString("\n\n")
	}
	return strings.TrimSpace(builder.String())
}

func roleTitle(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant":
		return "Assistant"
	case "system":
		return "System"
	default:
		return "User"
	}
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
