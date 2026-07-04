package tools

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"careerpilot-agent/internal/domain"
)

type ParseJDTool struct{}

type ParseJDInput struct {
	JDText string `json:"jd_text"`
}

func (t *ParseJDTool) Name() string {
	return "parse_jd"
}

func (t *ParseJDTool) Description() string {
	return "Extract responsibilities, skills, bonus items, and keywords from a job description."
}

func (t *ParseJDTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	payload, err := Decode[ParseJDInput](input)
	if err != nil {
		return nil, err
	}

	text := strings.TrimSpace(payload.JDText)
	lines := splitMeaningfulLines(text)
	keywords := extractKeywords(text)
	analysis := domain.JDAnalysis{
		Responsibilities: pickLines(lines, []string{"负责", "建设", "设计", "开发", "优化", "implement", "build", "design", "develop"}, 6),
		RequiredSkills:   pickSkills(text, false),
		BonusSkills:      pickSkills(text, true),
		Keywords:         keywords,
	}

	if len(analysis.Responsibilities) == 0 && text != "" {
		analysis.Responsibilities = append(analysis.Responsibilities, firstSentence(text))
	}
	return Encode(analysis)
}

func splitMeaningfulLines(text string) []string {
	parts := regexp.MustCompile(`[\r\n]+`).Split(text, -1)
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.Trim(part, "-•*0123456789.、 "))
		if part != "" {
			lines = append(lines, part)
		}
	}
	return lines
}

func pickLines(lines []string, markers []string, limit int) []string {
	var picked []string
	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, marker := range markers {
			if strings.Contains(lower, strings.ToLower(marker)) {
				picked = append(picked, line)
				break
			}
		}
		if len(picked) >= limit {
			break
		}
	}
	return picked
}

func pickSkills(text string, bonus bool) []string {
	catalog := []string{"Go", "Golang", "Python", "AI Agent", "Agent", "RAG", "LLM", "Tool Calling", "Function Calling", "Prompt", "Vector DB", "PostgreSQL", "Redis", "Docker", "Kubernetes", "FastAPI", "React", "TypeScript", "SSE", "OpenTelemetry", "Evaluation", "Evals", "Workflow", "Microservice"}
	lower := strings.ToLower(text)
	var skills []string
	for _, skill := range catalog {
		if strings.Contains(lower, strings.ToLower(skill)) {
			skills = append(skills, skill)
		}
	}
	if bonus {
		return filterSkillsByContext(text, skills, []string{"加分", "优先", "bonus", "preferred", "plus"})
	}
	bonusSet := map[string]bool{}
	for _, skill := range filterSkillsByContext(text, skills, []string{"加分", "优先", "bonus", "preferred", "plus"}) {
		bonusSet[skill] = true
	}
	var required []string
	for _, skill := range skills {
		if !bonusSet[skill] {
			required = append(required, skill)
		}
	}
	return unique(required)
}

func filterSkillsByContext(text string, skills []string, markers []string) []string {
	lines := splitMeaningfulLines(text)
	var matched []string
	inMarkedSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		contextMatched := false
		for _, marker := range markers {
			if strings.Contains(lower, strings.ToLower(marker)) {
				contextMatched = true
				break
			}
		}
		if strings.HasPrefix(trimmed, "#") {
			inMarkedSection = contextMatched
		}
		if !contextMatched && !inMarkedSection {
			continue
		}
		for _, skill := range skills {
			if strings.Contains(lower, strings.ToLower(skill)) {
				matched = append(matched, skill)
			}
		}
	}
	return unique(matched)
}

func extractKeywords(text string) []string {
	words := pickSkills(text, false)
	words = append(words, pickSkills(text, true)...)
	if strings.Contains(strings.ToLower(text), "agent") || strings.Contains(text, "智能体") {
		words = append(words, "AI Agent", "tool calling", "planning")
	}
	if strings.Contains(strings.ToLower(text), "rag") || strings.Contains(text, "检索") {
		words = append(words, "RAG", "retrieval", "evidence")
	}
	return unique(words)
}

func unique(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value != "" && !seen[key] {
			seen[key] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func firstSentence(text string) string {
	parts := regexp.MustCompile(`[。.!?\n]`).Split(text, 2)
	if len(parts) == 0 {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(parts[0])
}
