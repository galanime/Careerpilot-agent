package api_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"careerpilot-agent/internal/agent"
	"careerpilot-agent/internal/api"
	"careerpilot-agent/internal/domain"
	"careerpilot-agent/internal/memory"
	"careerpilot-agent/internal/opportunities"
	"careerpilot-agent/internal/storage"
	"careerpilot-agent/internal/tools"
)

func TestCreateRunIsAsyncAndEventsStreamTerminates(t *testing.T) {
	runtime := newRuntime()
	server := httptest.NewServer(api.NewServer(runtime).Routes())
	defer server.Close()

	body := []byte(`{"company_name":"DemoCorp","job_title":"AI Agent Engineer Intern","target_role":"AI Agent Engineer","jd_text":"负责 AI Agent 平台开发，要求 Go、RAG、Tool Calling、Evaluation 和后端工程能力。"}`)
	resp, err := http.Post(server.URL+"/api/runs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/runs returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 accepted, got %d", resp.StatusCode)
	}

	var created domain.Run
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" || created.Status != domain.RunStatusCreated {
		t.Fatalf("unexpected created run: %#v", created)
	}

	client := &http.Client{Timeout: 2 * time.Second}
	eventsResp, err := client.Get(server.URL + "/api/runs/" + created.ID + "/events")
	if err != nil {
		t.Fatalf("GET events returned error: %v", err)
	}
	defer eventsResp.Body.Close()
	if eventsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 events status, got %d", eventsResp.StatusCode)
	}

	scanner := bufio.NewScanner(eventsResp.Body)
	seenFinished := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "event: run_finished" {
			seenFinished = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan events: %v", err)
	}
	if !seenFinished {
		t.Fatalf("expected run_finished event")
	}
}

func TestOpportunityCanBeCreatedAndEvaluated(t *testing.T) {
	runtime := newRuntime()
	dir := t.TempDir()
	opportunityStore := opportunities.NewStore(filepath.Join(dir, "opportunities"), filepath.Join(dir, "pipeline.md"))
	server := httptest.NewServer(api.NewServer(runtime, opportunityStore).Routes())
	defer server.Close()

	body := []byte(`{"company_name":"DemoCorp","job_title":"AI Agent Engineer Intern","target_role":"AI Agent Engineer","location":"北京","url":"https://example.com/jobs/agent","jd_text":"北京实习岗位，负责 AI Agent 平台开发，要求 Go、RAG、Tool Calling、Evaluation 和后端工程能力。"}`)
	resp, err := http.Post(server.URL+"/api/opportunities", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/opportunities returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 created, got %d", resp.StatusCode)
	}

	var opportunity domain.Opportunity
	if err := json.NewDecoder(resp.Body).Decode(&opportunity); err != nil {
		t.Fatalf("decode opportunity response: %v", err)
	}
	if opportunity.ID == "" || opportunity.Status != domain.OpportunityStatusNew {
		t.Fatalf("unexpected opportunity: %#v", opportunity)
	}

	evaluateResp, err := http.Post(server.URL+"/api/opportunities/"+opportunity.ID+"/evaluate", "application/json", bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("POST evaluate returned error: %v", err)
	}
	defer evaluateResp.Body.Close()
	if evaluateResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 evaluate status, got %d", evaluateResp.StatusCode)
	}

	loaded, err := opportunityStore.Get(opportunity.ID)
	if err != nil {
		t.Fatalf("load opportunity: %v", err)
	}
	if loaded.ReportRunID == "" || loaded.Decision == "" {
		t.Fatalf("expected opportunity to be marked scored, got %#v", loaded)
	}
}

func TestLivenessEndpoint(t *testing.T) {
	runtime := newRuntime()
	dir := t.TempDir()
	opportunityStore := opportunities.NewStore(filepath.Join(dir, "opportunities"), filepath.Join(dir, "pipeline.md"))
	server := httptest.NewServer(api.NewServer(runtime, opportunityStore).Routes())
	defer server.Close()

	jobPage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<h1>AI Agent Engineer</h1><button>Apply</button><p>Requirements</p>"))
	}))
	defer jobPage.Close()

	body := []byte(`{"url":"` + jobPage.URL + `"}`)
	resp, err := http.Post(server.URL+"/api/liveness", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/liveness returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", resp.StatusCode)
	}
	var report domain.LivenessReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		t.Fatalf("decode liveness report: %v", err)
	}
	if report.Status != domain.LivenessActive {
		t.Fatalf("expected active liveness, got %#v", report)
	}
}

func TestScanEndpointAddsOpportunities(t *testing.T) {
	runtime := newRuntime()
	dir := t.TempDir()
	opportunityStore := opportunities.NewStore(filepath.Join(dir, "opportunities"), filepath.Join(dir, "pipeline.md"))
	server := httptest.NewServer(api.NewServer(runtime, opportunityStore).Routes())
	defer server.Close()

	jobPage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<h1>Careers</h1><button>Apply</button><p>Job description</p>"))
	}))
	defer jobPage.Close()

	body := []byte(`{"target_role":"AI Agent Engineer","companies":[{"name":"DemoCorp","provider":"direct","careers_url":"` + jobPage.URL + `","enabled":true}]}`)
	resp, err := http.Post(server.URL+"/api/scan", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/scan returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", resp.StatusCode)
	}
	var result domain.ScanResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode scan result: %v", err)
	}
	if result.Added != 1 {
		t.Fatalf("expected one added job, got %#v", result)
	}
}

func newRuntime() *agent.Runtime {
	store := memory.NewStore([]domain.EvidenceItem{{
		Project: "LifeHelper",
		Title:   "Agent runtime with tool calling and trace",
		Tags:    []string{"AI Agent", "tool calling", "trace", "evaluation", "Go"},
		Details: []string{"Implemented task decomposition and observable tool execution."},
	}})
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
