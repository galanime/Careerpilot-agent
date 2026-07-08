package agent_test

import (
	"context"
	"testing"
	"time"

	"careerpilot-agent/internal/agent"
	"careerpilot-agent/internal/domain"
	"careerpilot-agent/internal/memory"
	"careerpilot-agent/internal/storage"
	"careerpilot-agent/internal/tools"
)

func TestRuntimeStartRun(t *testing.T) {
	runtime := newTestRuntime()
	run, err := runtime.StartRun(context.Background(), testInput())
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}
	assertCompletedRun(t, run)
}

func TestRuntimeStartRunAsyncPublishesEvents(t *testing.T) {
	runtime := newTestRuntime()
	run, err := runtime.StartRunAsync(context.Background(), testInput())
	if err != nil {
		t.Fatalf("StartRunAsync returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	events := runtime.SubscribeEvents(ctx, run.ID)
	var terminal agent.Event
	for event := range events {
		if event.Terminal() {
			terminal = event
			break
		}
	}
	if terminal.Type != agent.EventRunFinished {
		t.Fatalf("expected run_finished event, got %#v", terminal)
	}

	loaded, err := runtime.GetRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}
	assertCompletedRun(t, loaded)
}

func newTestRuntime() *agent.Runtime {
	store := memory.NewStore([]domain.EvidenceItem{
		{
			Project: "LifeHelper",
			Title:   "Agent runtime with tool calling and trace",
			Tags:    []string{"AI Agent", "tool calling", "trace", "evaluation", "Go"},
			Details: []string{"Implemented task decomposition and observable tool execution."},
		},
	})
	registry := tools.NewRegistry()
	registry.Register(&tools.ParseJDTool{})
	registry.Register(tools.NewSearchEvidenceTool(store))
	registry.Register(&tools.MatchScoringTool{})
	registry.Register(&tools.EvaluateOpportunityTool{})
	registry.Register(tools.NewGenerateMaterialsTool(nil))
	registry.Register(&tools.EvaluateOutputTool{})
	registry.Register(&tools.BuildApplicationPlanTool{})
	return agent.NewRuntime(storage.NewStore(), registry)
}

func testInput() domain.RunInput {
	return domain.RunInput{
		CompanyName: "DemoCorp",
		JobTitle:    "AI Agent Engineer Intern",
		TargetRole:  "AI Agent Engineer",
		JDText:      "负责 AI Agent 平台开发，要求 Go、RAG、Tool Calling、Evaluation 和后端工程能力。",
	}
}

func assertCompletedRun(t *testing.T, run domain.Run) {
	t.Helper()
	if run.Status != domain.RunStatusDone {
		t.Fatalf("expected done status, got %s", run.Status)
	}
	if len(run.Steps) != 8 {
		t.Fatalf("expected 8 steps, got %d", len(run.Steps))
	}
	if len(run.Artifacts) != 6 {
		t.Fatalf("expected 6 artifacts, got %d", len(run.Artifacts))
	}
}
