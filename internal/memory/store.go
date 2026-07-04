package memory

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"careerpilot-agent/internal/domain"
)

type Store struct {
	items []domain.EvidenceItem
}

func LoadStore(path string) (*Store, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	items, err := parseEvidence(file)
	if err != nil {
		return nil, err
	}
	return &Store{items: items}, nil
}

func NewStore(items []domain.EvidenceItem) *Store {
	return &Store{items: items}
}

func (s *Store) Search(ctx context.Context, keywords []string, limit int) domain.EvidenceSearchResult {
	type scored struct {
		item  domain.EvidenceItem
		score int
	}

	var scoredItems []scored
	for _, item := range s.items {
		select {
		case <-ctx.Done():
			return domain.EvidenceSearchResult{Query: keywords, Items: nil, Summary: ctx.Err().Error()}
		default:
		}

		haystack := strings.ToLower(item.Project + " " + item.Title + " " + strings.Join(item.Tags, " ") + " " + strings.Join(item.Details, " "))
		score := 0
		for _, keyword := range keywords {
			keyword = strings.ToLower(strings.TrimSpace(keyword))
			if keyword != "" && strings.Contains(haystack, keyword) {
				score++
			}
		}
		if score > 0 {
			scoredItems = append(scoredItems, scored{item: item, score: score})
		}
	}

	sort.Slice(scoredItems, func(i, j int) bool {
		if scoredItems[i].score == scoredItems[j].score {
			return scoredItems[i].item.Project < scoredItems[j].item.Project
		}
		return scoredItems[i].score > scoredItems[j].score
	})

	if limit <= 0 || limit > len(scoredItems) {
		limit = len(scoredItems)
	}

	items := make([]domain.EvidenceItem, 0, limit)
	for _, scoredItem := range scoredItems[:limit] {
		items = append(items, scoredItem.item)
	}

	return domain.EvidenceSearchResult{
		Query:   keywords,
		Items:   items,
		Summary: fmt.Sprintf("matched %d evidence items from %d keywords", len(items), len(keywords)),
	}
}

func parseEvidence(file *os.File) ([]domain.EvidenceItem, error) {
	scanner := bufio.NewScanner(file)
	var items []domain.EvidenceItem
	var current *domain.EvidenceItem
	section := ""

	flush := func() {
		if current != nil && current.Project != "" {
			items = append(items, *current)
		}
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "- project:"):
			flush()
			current = &domain.EvidenceItem{Project: cleanYAMLValue(strings.TrimPrefix(trimmed, "- project:"))}
			section = ""
		case current == nil:
			continue
		case strings.HasPrefix(trimmed, "title:"):
			current.Title = cleanYAMLValue(strings.TrimPrefix(trimmed, "title:"))
		case strings.HasPrefix(trimmed, "tags:"):
			current.Tags = splitInlineList(strings.TrimPrefix(trimmed, "tags:"))
			section = "tags"
		case strings.HasPrefix(trimmed, "details:"):
			section = "details"
		case strings.HasPrefix(trimmed, "-"):
			value := cleanYAMLValue(strings.TrimPrefix(trimmed, "-"))
			if section == "details" {
				current.Details = append(current.Details, value)
			} else if section == "tags" {
				current.Tags = append(current.Tags, value)
			}
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func splitInlineList(raw string) []string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := cleanYAMLValue(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func cleanYAMLValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"")
	return value
}
