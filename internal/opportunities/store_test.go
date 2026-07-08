package opportunities_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"careerpilot-agent/internal/domain"
	"careerpilot-agent/internal/opportunities"
)

func TestStoreAddDeduplicatesAndWritesPipeline(t *testing.T) {
	dir := t.TempDir()
	store := opportunities.NewStore(filepath.Join(dir, "opportunities"), filepath.Join(dir, "pipeline.md"))
	input := domain.OpportunityInput{
		CompanyName: "DemoCorp",
		JobTitle:    "AI Agent Engineer Intern",
		TargetRole:  "AI Agent Engineer",
		URL:         "https://example.com/jobs/1",
		JDText:      "负责 AI Agent 平台开发，要求 Go、Tool Calling 和后端能力。",
	}

	created, err := store.Add(input)
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if created.ID == "" || created.Status != domain.OpportunityStatusNew {
		t.Fatalf("unexpected created opportunity: %#v", created)
	}

	if _, err := store.Add(input); !errors.Is(err, opportunities.ErrDuplicateOpportunity) {
		t.Fatalf("expected duplicate error, got %v", err)
	}

	pipeline, err := os.ReadFile(filepath.Join(dir, "pipeline.md"))
	if err != nil {
		t.Fatalf("read pipeline: %v", err)
	}
	if !strings.Contains(string(pipeline), created.ID) || !strings.Contains(string(pipeline), "DemoCorp") {
		t.Fatalf("pipeline missing opportunity:\n%s", string(pipeline))
	}
}

func TestStoreMarkScoredUpdatesDecisionStatus(t *testing.T) {
	dir := t.TempDir()
	store := opportunities.NewStore(filepath.Join(dir, "opportunities"), filepath.Join(dir, "pipeline.md"))
	created, err := store.Add(domain.OpportunityInput{
		CompanyName: "DemoCorp",
		JobTitle:    "AI Agent Engineer Intern",
		TargetRole:  "AI Agent Engineer",
		JDText:      "负责 AI Agent 平台开发。",
	})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	updated, err := store.MarkScored(created.ID, 82.5, string(domain.OpportunityDecisionApply), "run_123")
	if err != nil {
		t.Fatalf("MarkScored returned error: %v", err)
	}
	if updated.Status != domain.OpportunityStatusReview || updated.ReportRunID != "run_123" {
		t.Fatalf("unexpected updated opportunity: %#v", updated)
	}
}
