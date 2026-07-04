package storage_test

import (
	"context"
	"testing"
	"time"

	"careerpilot-agent/internal/domain"
	"careerpilot-agent/internal/storage"
)

func TestSQLiteStorePersistsRun(t *testing.T) {
	store, err := storage.NewSQLiteStore(context.Background(), t.TempDir()+"/careerpilot.db")
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second)
	run := domain.Run{
		ID: "run_test",
		Input: domain.RunInput{
			CompanyName: "DemoCorp",
			JobTitle:    "AI Agent Engineer Intern",
			TargetRole:  "AI Agent Engineer",
			JDText:      "需要 Go、RAG 和 Agent 工程经验。",
		},
		Status:    domain.RunStatusDone,
		CreatedAt: now,
		UpdatedAt: now,
		Steps: []domain.Step{{
			ID:        "step_test",
			RunID:     "run_test",
			Type:      domain.StepTypeTool,
			Name:      "parse_jd",
			Status:    domain.StepStatusDone,
			Input:     map[string]any{"jd_text": "demo"},
			Output:    map[string]any{"keywords": []string{"Go"}},
			StartedAt: now,
			EndedAt:   now,
		}},
		Artifacts: []domain.Artifact{{
			ID:        "artifact_test",
			RunID:     "run_test",
			Type:      "match_report",
			Title:     "岗位匹配报告",
			Content:   "ok",
			Metadata:  map[string]any{"score": 90},
			CreatedAt: now,
		}},
	}

	if err := store.SaveRun(run); err != nil {
		t.Fatalf("SaveRun returned error: %v", err)
	}
	loaded, err := store.GetRun(run.ID)
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}
	if loaded.ID != run.ID || loaded.Status != domain.RunStatusDone {
		t.Fatalf("unexpected loaded run: %#v", loaded)
	}
	if len(loaded.Steps) != 1 || loaded.Steps[0].Name != "parse_jd" {
		t.Fatalf("unexpected loaded steps: %#v", loaded.Steps)
	}
	if len(loaded.Artifacts) != 1 || loaded.Artifacts[0].Type != "match_report" {
		t.Fatalf("unexpected loaded artifacts: %#v", loaded.Artifacts)
	}

	runs, err := store.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != run.ID {
		t.Fatalf("unexpected runs list: %#v", runs)
	}
}
