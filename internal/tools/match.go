package tools

import (
	"context"
	"encoding/json"

	"careerpilot-agent/internal/domain"
	"careerpilot-agent/internal/memory"
)

type SearchEvidenceTool struct {
	store *memory.Store
}

type SearchEvidenceInput struct {
	Keywords []string `json:"keywords"`
	Limit    int      `json:"limit"`
}

func NewSearchEvidenceTool(store *memory.Store) *SearchEvidenceTool {
	return &SearchEvidenceTool{store: store}
}

func (t *SearchEvidenceTool) Name() string {
	return "search_evidence"
}

func (t *SearchEvidenceTool) Description() string {
	return "Search candidate evidence and return grounded project examples."
}

func (t *SearchEvidenceTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	payload, err := Decode[SearchEvidenceInput](input)
	if err != nil {
		return nil, err
	}
	if payload.Limit == 0 {
		payload.Limit = 5
	}
	result := t.store.Search(ctx, payload.Keywords, payload.Limit)
	return Encode(result)
}

type MatchScoringTool struct{}

type MatchScoringInput struct {
	Analysis domain.JDAnalysis           `json:"analysis"`
	Evidence domain.EvidenceSearchResult `json:"evidence"`
}

func (t *MatchScoringTool) Name() string {
	return "score_match"
}

func (t *MatchScoringTool) Description() string {
	return "Score how well candidate evidence matches the parsed job description."
}

func (t *MatchScoringTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	payload, err := Decode[MatchScoringInput](input)
	if err != nil {
		return nil, err
	}

	covered := map[string]bool{}
	for _, item := range payload.Evidence.Items {
		text := item.Project + " " + item.Title
		for _, tag := range item.Tags {
			text += " " + tag
		}
		for _, detail := range item.Details {
			text += " " + detail
		}
		for _, keyword := range payload.Analysis.Keywords {
			if containsFold(text, keyword) {
				covered[keyword] = true
			}
		}
	}

	keywordCount := len(payload.Analysis.Keywords)
	score := 50
	if keywordCount > 0 {
		score = 40 + int(float64(len(covered))/float64(keywordCount)*55)
	}
	if len(payload.Evidence.Items) == 0 {
		score = 35
	}
	if score > 95 {
		score = 95
	}

	report := domain.MatchReport{
		Score: score,
		Gaps:  []string{},
	}
	for keyword := range covered {
		report.StrongMatches = append(report.StrongMatches, keyword)
	}
	for _, keyword := range payload.Analysis.Keywords {
		if !covered[keyword] {
			report.Gaps = append(report.Gaps, keyword)
		}
	}
	for _, item := range payload.Evidence.Items {
		report.RecommendedUse = append(report.RecommendedUse, item.Project+": "+item.Title)
	}
	if len(report.Gaps) > 0 {
		report.MediumMatches = append(report.MediumMatches, "部分关键词缺少直接证据，建议在项目描述中用真实实现细节补强")
	}
	return Encode(report)
}
