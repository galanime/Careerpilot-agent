package memory

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"careerpilot-agent/internal/domain"
)

func LoadJobHuntProjects(path string) ([]domain.EvidenceItem, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var items []domain.EvidenceItem
	var current *domain.EvidenceItem
	section := ""

	flush := func() {
		if current != nil && (current.Project != "" || current.Title != "") {
			if current.Project == "" {
				current.Project = current.Title
			}
			items = append(items, *current)
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "- id:"):
			flush()
			current = &domain.EvidenceItem{Project: cleanYAMLValue(strings.TrimPrefix(trimmed, "- id:"))}
			section = ""
		case current == nil:
			continue
		case strings.HasPrefix(trimmed, "canonical_name:"):
			name := cleanYAMLValue(strings.TrimPrefix(trimmed, "canonical_name:"))
			current.Project = name
			current.Title = name
		case strings.HasPrefix(trimmed, "title_variants:"):
			section = ""
		case strings.HasPrefix(trimmed, "tags:"):
			section = "tags"
		case strings.HasPrefix(trimmed, "tech_stack:"):
			section = "tags"
		case strings.HasPrefix(trimmed, "strengths:"):
			section = "details"
		case strings.HasPrefix(trimmed, "evidence_paths:"):
			section = "details"
		case strings.HasPrefix(trimmed, "project_300char:"):
			current.Details = append(current.Details, cleanYAMLValue(strings.TrimPrefix(trimmed, "project_300char:")))
		case strings.HasPrefix(trimmed, "caution:"):
			section = ""
		case strings.HasPrefix(trimmed, "-"):
			value := cleanYAMLValue(strings.TrimPrefix(trimmed, "-"))
			if value == "" {
				continue
			}
			if section == "tags" {
				current.Tags = append(current.Tags, value)
			}
			if section == "details" {
				current.Details = append(current.Details, value)
			}
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return dedupeEvidence(items), nil
}

func dedupeEvidence(items []domain.EvidenceItem) []domain.EvidenceItem {
	seen := map[string]bool{}
	out := make([]domain.EvidenceItem, 0, len(items))
	for _, item := range items {
		key := strings.ToLower(item.Project + "|" + item.Title)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}
