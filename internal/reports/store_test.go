package reports_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"careerpilot-agent/internal/domain"
	"careerpilot-agent/internal/reports"
)

func TestSaveRunReportWritesReportAndApplicationsTracker(t *testing.T) {
	dir := t.TempDir()
	store := reports.NewStore(filepath.Join(dir, "reports"), filepath.Join(dir, "data", "applications.md"))
	run := domain.Run{
		ID: "run_123",
		Artifacts: []domain.Artifact{{
			Type:      "ag_report",
			Content:   "# Evaluation\n\n## A) Role Summary\n",
			CreatedAt: time.Now(),
		}},
	}
	opportunity := domain.Opportunity{
		CompanyName: "DemoCorp",
		JobTitle:    "AI Agent Engineer Intern",
		Status:      domain.OpportunityStatusReview,
		Score:       82.5,
		Decision:    "apply",
	}

	saved, err := store.SaveRunReport(run, opportunity)
	if err != nil {
		t.Fatalf("SaveRunReport returned error: %v", err)
	}
	reportBytes, err := os.ReadFile(saved.ReportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !strings.Contains(string(reportBytes), "Role Summary") {
		t.Fatalf("unexpected report content: %s", string(reportBytes))
	}
	trackerBytes, err := os.ReadFile(saved.ApplicationPath)
	if err != nil {
		t.Fatalf("read tracker: %v", err)
	}
	if !strings.Contains(string(trackerBytes), "DemoCorp") || !strings.Contains(string(trackerBytes), "run_123") {
		t.Fatalf("unexpected tracker content: %s", string(trackerBytes))
	}
}
