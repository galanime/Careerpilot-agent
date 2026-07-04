package agent_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"careerpilot-agent/internal/agent"
	"careerpilot-agent/internal/domain"
	"careerpilot-agent/internal/memory"
	"careerpilot-agent/internal/storage"
	"careerpilot-agent/internal/tools"
)

func TestGoldenTraceForExampleJD(t *testing.T) {
	runtime := newGoldenRuntime(t)
	jdText := readGoldenFile(t, "examples", "jds", "ai_agent_engineer.md")

	run, err := runtime.StartRun(context.Background(), domain.RunInput{
		CompanyName: "ByteDance",
		JobTitle:    "AI Agent Engineer Intern",
		TargetRole:  "AI Agent Engineer",
		JDText:      jdText,
	})
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}

	if run.Status != domain.RunStatusDone {
		t.Fatalf("expected done status, got %s", run.Status)
	}

	stepNames := make([]string, 0, len(run.Steps))
	for _, step := range run.Steps {
		stepNames = append(stepNames, step.Name)
		if step.Status != domain.StepStatusDone {
			t.Fatalf("step %s expected done status, got %s: %s", step.Name, step.Status, step.Error)
		}
	}
	expectedSteps := []string{"planner", "parse_jd", "search_evidence", "score_match", "generate_materials", "evaluate_output"}
	if !reflect.DeepEqual(stepNames, expectedSteps) {
		t.Fatalf("unexpected step order:\nwant %#v\n got %#v", expectedSteps, stepNames)
	}

	artifactTypes := make([]string, 0, len(run.Artifacts))
	artifactsByType := map[string]domain.Artifact{}
	for _, artifact := range run.Artifacts {
		artifactTypes = append(artifactTypes, artifact.Type)
		artifactsByType[artifact.Type] = artifact
	}
	expectedArtifacts := []string{"jd_analysis", "match_report", "application_materials", "eval_report"}
	if !reflect.DeepEqual(artifactTypes, expectedArtifacts) {
		t.Fatalf("unexpected artifact order:\nwant %#v\n got %#v", expectedArtifacts, artifactTypes)
	}

	var analysis domain.JDAnalysis
	decodeArtifact(t, artifactsByType["jd_analysis"], &analysis)
	assertContains(t, analysis.Keywords, "AI Agent")
	assertContains(t, analysis.Keywords, "Tool Calling")
	assertContains(t, analysis.Keywords, "RAG")
	assertContains(t, analysis.BonusSkills, "React")
	assertContains(t, analysis.BonusSkills, "SSE")

	var match domain.MatchReport
	decodeArtifact(t, artifactsByType["match_report"], &match)
	if match.Score < 65 {
		t.Fatalf("expected match score >= 65, got %d", match.Score)
	}
	if len(match.RecommendedUse) == 0 {
		t.Fatalf("expected recommended evidence projects")
	}
	assertContains(t, match.RecommendedUse, "LifeHelper: AI Agent task decomposition and traceable tool execution")

	materials := artifactsByType["application_materials"].Content
	assertTextContains(t, materials, "LifeHelper")
	assertTextContains(t, materials, "Agent")
	assertTextContains(t, materials, "面试问答")

	var evalReport domain.EvalReport
	decodeArtifact(t, artifactsByType["eval_report"], &evalReport)
	if !evalReport.Passed {
		t.Fatalf("expected eval report to pass: %#v", evalReport)
	}
	if evalReport.KeywordCoverage < 0.45 {
		t.Fatalf("expected keyword coverage >= 0.45, got %.2f", evalReport.KeywordCoverage)
	}
	if len(evalReport.UnsupportedClaims) > 0 {
		t.Fatalf("expected no unsupported claims, got %#v", evalReport.UnsupportedClaims)
	}
}

func TestEvalFlagsMissingEvidence(t *testing.T) {
	runtime := newRuntimeWithEvidence(nil)
	run, err := runtime.StartRun(context.Background(), domain.RunInput{
		CompanyName: "DemoCorp",
		JobTitle:    "AI Agent Engineer Intern",
		TargetRole:  "AI Agent Engineer",
		JDText:      "负责 AI Agent 平台开发，要求 Go、RAG、Tool Calling、Evaluation 和后端工程能力。",
	})
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}

	var evalReport domain.EvalReport
	decodeArtifact(t, artifactByType(t, run, "eval_report"), &evalReport)
	if evalReport.Passed {
		t.Fatalf("expected eval report to fail without evidence: %#v", evalReport)
	}
	assertContains(t, evalReport.UnsupportedClaims, "缺少候选人项目 evidence，不能生成可投递材料")
}

func newGoldenRuntime(t *testing.T) *agent.Runtime {
	t.Helper()
	evidenceStore, err := memory.LoadStore(readGoldenPath(t, "data", "evidence", "projects.yaml"))
	if err != nil {
		t.Fatalf("load evidence: %v", err)
	}
	return newRuntimeWithStore(evidenceStore)
}

func newRuntimeWithEvidence(evidence []domain.EvidenceItem) *agent.Runtime {
	return newRuntimeWithStore(memory.NewStore(evidence))
}

func newRuntimeWithStore(store *memory.Store) *agent.Runtime {
	registry := tools.NewRegistry()
	registry.Register(&tools.ParseJDTool{})
	registry.Register(tools.NewSearchEvidenceTool(store))
	registry.Register(&tools.MatchScoringTool{})
	registry.Register(tools.NewGenerateMaterialsTool(nil))
	registry.Register(&tools.EvaluateOutputTool{})
	return agent.NewRuntime(storage.NewStore(), registry)
}

func readGoldenFile(t *testing.T, parts ...string) string {
	t.Helper()
	content, err := os.ReadFile(readGoldenPath(t, parts...))
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}
	return string(content)
}

func readGoldenPath(t *testing.T, parts ...string) string {
	t.Helper()
	root := filepath.Join("..", "..")
	return filepath.Join(append([]string{root}, parts...)...)
}

func artifactByType(t *testing.T, run domain.Run, artifactType string) domain.Artifact {
	t.Helper()
	for _, artifact := range run.Artifacts {
		if artifact.Type == artifactType {
			return artifact
		}
	}
	t.Fatalf("artifact %s not found", artifactType)
	return domain.Artifact{}
}

func decodeArtifact(t *testing.T, artifact domain.Artifact, target any) {
	t.Helper()
	if artifact.Content == "" {
		t.Fatalf("artifact %s has empty content", artifact.Type)
	}
	if err := json.Unmarshal([]byte(artifact.Content), target); err != nil {
		t.Fatalf("decode artifact %s: %v\n%s", artifact.Type, err, artifact.Content)
	}
}

func assertContains(t *testing.T, values []string, expected string) {
	t.Helper()
	for _, value := range values {
		if value == expected {
			return
		}
	}
	t.Fatalf("expected %#v to contain %q", values, expected)
}

func assertTextContains(t *testing.T, text string, expected string) {
	t.Helper()
	if !strings.Contains(text, expected) {
		t.Fatalf("expected text to contain %q:\n%s", expected, text)
	}
}
