package api_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"careerpilot-agent/internal/agent"
	"careerpilot-agent/internal/api"
	"careerpilot-agent/internal/domain"
	"careerpilot-agent/internal/memory"
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
	registry.Register(tools.NewGenerateMaterialsTool(nil))
	registry.Register(&tools.EvaluateOutputTool{})
	return agent.NewRuntime(storage.NewStore(), registry)
}
