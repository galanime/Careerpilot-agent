package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"careerpilot-agent/internal/agent"
	"careerpilot-agent/internal/api"
	"careerpilot-agent/internal/llm"
	"careerpilot-agent/internal/memory"
	"careerpilot-agent/internal/opportunities"
	"careerpilot-agent/internal/storage"
	"careerpilot-agent/internal/tools"
)

func main() {
	evidencePath := getenv("CAREERPILOT_EVIDENCE_PATH", "data/evidence/projects.yaml")
	jobHuntProjectsPath := getenv("CAREERPILOT_JOB_HUNT_PROJECTS_PATH", "../job-hunt-kb/data/materials/projects.yaml")
	addr := getenv("CAREERPILOT_ADDR", ":8788")
	dbPath := getenv("CAREERPILOT_DB_PATH", "careerpilot.db")
	opportunityDir := getenv("CAREERPILOT_OPPORTUNITY_DIR", "data/opportunities")
	pipelinePath := getenv("CAREERPILOT_PIPELINE_PATH", "data/pipeline.md")

	memoryStore, err := memory.LoadStore(evidencePath)
	if err != nil {
		log.Fatalf("load evidence store: %v", err)
	}
	if jobHuntProjectsPath != "" {
		jobHuntEvidence, err := memory.LoadJobHuntProjects(jobHuntProjectsPath)
		if err != nil {
			log.Printf("job-hunt-kb project bridge skipped: %v", err)
		} else {
			memoryStore.Append(jobHuntEvidence)
			log.Printf("loaded %d job-hunt-kb project evidence items", len(jobHuntEvidence))
		}
	}

	runStore, err := storage.NewSQLiteStore(context.Background(), dbPath)
	if err != nil {
		log.Fatalf("open sqlite store: %v", err)
	}
	defer runStore.Close()

	provider, err := llm.NewProvider(llm.Config{
		Provider:       getenv("CAREERPILOT_LLM_PROVIDER", "fake"),
		APIKey:         firstEnv("CAREERPILOT_LLM_API_KEY", "ANTHROPIC_API_KEY", "OPENAI_API_KEY"),
		BaseURL:        getenv("CAREERPILOT_LLM_BASE_URL", ""),
		Model:          getenv("CAREERPILOT_LLM_MODEL", ""),
		MaxTokens:      getenvInt("CAREERPILOT_LLM_MAX_TOKENS", 4096),
		EnableThinking: getenvBoolPtr("CAREERPILOT_LLM_THINKING"),
		Effort:         getenv("CAREERPILOT_LLM_EFFORT", "high"),
	})
	if err != nil {
		log.Printf("llm provider unavailable, falling back to fake provider: %v", err)
		provider = llm.NewFakeProvider()
	}

	registry := tools.NewRegistry()
	registry.Register(&tools.ParseJDTool{})
	registry.Register(tools.NewSearchEvidenceTool(memoryStore))
	registry.Register(&tools.MatchScoringTool{})
	registry.Register(&tools.EvaluateOpportunityTool{})
	registry.Register(tools.NewGenerateMaterialsTool(provider))
	registry.Register(&tools.EvaluateOutputTool{})
	registry.Register(&tools.BuildApplicationPlanTool{})

	runtime := agent.NewRuntime(runStore, registry)
	opportunityStore := opportunities.NewStore(opportunityDir, pipelinePath)
	server := &http.Server{
		Addr:              addr,
		Handler:           api.NewServer(runtime, opportunityStore).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("CareerPilot Agent listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvBoolPtr(key string) *bool {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil
	}
	return &parsed
}
