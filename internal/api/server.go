package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"careerpilot-agent/internal/agent"
	"careerpilot-agent/internal/domain"
	"careerpilot-agent/internal/opportunities"
	"careerpilot-agent/internal/scanner"
	"careerpilot-agent/internal/storage"
)

type Server struct {
	runtime       *agent.Runtime
	opportunities *opportunities.Store
	scanner       *scanner.Scanner
}

func NewServer(runtime *agent.Runtime, opportunityStores ...*opportunities.Store) *Server {
	var opportunityStore *opportunities.Store
	if len(opportunityStores) > 0 {
		opportunityStore = opportunityStores[0]
	}
	var jobScanner *scanner.Scanner
	if opportunityStore != nil {
		jobScanner = scanner.New(opportunityStore)
	}
	return &Server{runtime: runtime, opportunities: opportunityStore, scanner: jobScanner}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/runs", s.handleListRuns)
	mux.HandleFunc("POST /api/runs", s.handleCreateRun)
	mux.HandleFunc("GET /api/runs/", s.handleRunSubresource)
	mux.HandleFunc("GET /api/opportunities", s.handleListOpportunities)
	mux.HandleFunc("POST /api/opportunities", s.handleCreateOpportunity)
	mux.HandleFunc("GET /api/opportunities/", s.handleOpportunitySubresource)
	mux.HandleFunc("POST /api/opportunities/", s.handleOpportunitySubresource)
	mux.HandleFunc("POST /api/scan", s.handleScan)
	mux.HandleFunc("POST /api/liveness", s.handleLiveness)
	return withCORS(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "time": time.Now().UTC()})
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := s.runtime.ListRuns(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

func (s *Server) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	var input domain.RunInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(input.JDText) == "" {
		writeError(w, http.StatusBadRequest, "jd_text is required")
		return
	}

	run, err := s.runtime.StartRunAsync(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"run": run, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}

func (s *Server) handleRunSubresource(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/runs/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "run id is required")
		return
	}

	if len(parts) == 2 && parts[1] == "events" {
		s.writeEventStream(w, r, parts[0])
		return
	}

	run, err := s.runtime.GetRun(r.Context(), parts[0])
	if err != nil {
		if errors.Is(err, storage.ErrRunNotFound) {
			writeError(w, http.StatusNotFound, "run not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(parts) == 2 && parts[1] == "artifacts" {
		writeJSON(w, http.StatusOK, map[string]any{"artifacts": run.Artifacts})
		return
	}
	if len(parts) == 1 {
		writeJSON(w, http.StatusOK, run)
		return
	}
	writeError(w, http.StatusNotFound, "unknown run endpoint")
}

func (s *Server) handleListOpportunities(w http.ResponseWriter, r *http.Request) {
	if s.opportunities == nil {
		writeError(w, http.StatusServiceUnavailable, "opportunity store is not configured")
		return
	}
	items, err := s.opportunities.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"opportunities": items})
}

func (s *Server) handleCreateOpportunity(w http.ResponseWriter, r *http.Request) {
	if s.opportunities == nil {
		writeError(w, http.StatusServiceUnavailable, "opportunity store is not configured")
		return
	}
	var input domain.OpportunityInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	opportunity, err := s.opportunities.Add(input)
	if err != nil {
		if errors.Is(err, opportunities.ErrDuplicateOpportunity) {
			writeJSON(w, http.StatusConflict, map[string]any{"opportunity": opportunity, "error": "duplicate opportunity"})
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, opportunity)
}

func (s *Server) handleOpportunitySubresource(w http.ResponseWriter, r *http.Request) {
	if s.opportunities == nil {
		writeError(w, http.StatusServiceUnavailable, "opportunity store is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/opportunities/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "opportunity id is required")
		return
	}

	opportunity, err := s.opportunities.Get(parts[0])
	if err != nil {
		if errors.Is(err, opportunities.ErrOpportunityNotFound) {
			writeError(w, http.StatusNotFound, "opportunity not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(parts) == 1 && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, opportunity)
		return
	}
	if len(parts) == 2 && parts[1] == "evaluate" && r.Method == http.MethodPost {
		s.handleEvaluateOpportunity(w, r, opportunity)
		return
	}
	if len(parts) == 2 && parts[1] == "status" && r.Method == http.MethodPost {
		s.handleUpdateOpportunityStatus(w, r, opportunity)
		return
	}
	writeError(w, http.StatusNotFound, "unknown opportunity endpoint")
}

func (s *Server) handleEvaluateOpportunity(w http.ResponseWriter, r *http.Request, opportunity domain.Opportunity) {
	run, err := s.runtime.ExecuteRun(r.Context(), domain.RunInput{
		CompanyName:   opportunity.CompanyName,
		JobTitle:      opportunity.JobTitle,
		TargetRole:    opportunity.TargetRole,
		JDText:        opportunity.JDText,
		OpportunityID: opportunity.ID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"run": run, "error": err.Error()})
		return
	}
	score, decision := extractOpportunityDecision(run)
	if _, err := s.opportunities.MarkScored(opportunity.ID, score, decision, run.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"run": run, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, run)
}

type updateOpportunityStatusRequest struct {
	Status domain.OpportunityStatus `json:"status"`
	Notes  []string                 `json:"notes,omitempty"`
}

func (s *Server) handleUpdateOpportunityStatus(w http.ResponseWriter, r *http.Request, opportunity domain.Opportunity) {
	var input updateOpportunityStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !validOpportunityStatus(input.Status) {
		writeError(w, http.StatusBadRequest, "invalid opportunity status")
		return
	}
	updated, err := s.opportunities.UpdateStatus(opportunity.ID, input.Status, input.Notes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if s.scanner == nil {
		writeError(w, http.StatusServiceUnavailable, "scanner is not configured")
		return
	}
	var input domain.ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(input.Companies) == 0 {
		writeError(w, http.StatusBadRequest, "companies is required")
		return
	}
	result := s.scanner.Scan(r.Context(), input)
	writeJSON(w, http.StatusOK, result)
}

type livenessRequest struct {
	URL string `json:"url"`
}

func (s *Server) handleLiveness(w http.ResponseWriter, r *http.Request) {
	if s.scanner == nil {
		writeError(w, http.StatusServiceUnavailable, "scanner is not configured")
		return
	}
	var input livenessRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(input.URL) == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	writeJSON(w, http.StatusOK, s.scanner.CheckLiveness(r.Context(), input.URL))
}

func validOpportunityStatus(status domain.OpportunityStatus) bool {
	switch status {
	case domain.OpportunityStatusNew,
		domain.OpportunityStatusScored,
		domain.OpportunityStatusReview,
		domain.OpportunityStatusApplied,
		domain.OpportunityStatusInterview,
		domain.OpportunityStatusRejected,
		domain.OpportunityStatusArchived,
		domain.OpportunityStatusDoNotApply:
		return true
	default:
		return false
	}
}

func extractOpportunityDecision(run domain.Run) (float64, string) {
	for _, artifact := range run.Artifacts {
		if artifact.Type != "opportunity_evaluation" {
			continue
		}
		var evaluation domain.OpportunityEvaluation
		if err := json.Unmarshal([]byte(artifact.Content), &evaluation); err != nil {
			return 0, ""
		}
		return evaluation.OverallScore, string(evaluation.Decision)
	}
	return 0, ""
}

func (s *Server) writeEventStream(w http.ResponseWriter, r *http.Request, runID string) {
	if _, err := s.runtime.GetRun(r.Context(), runID); err != nil {
		if errors.Is(err, storage.ErrRunNotFound) {
			writeError(w, http.StatusNotFound, "run not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	for event := range s.runtime.SubscribeEvents(r.Context(), runID) {
		writeSSE(w, event)
		flusher.Flush()
		if event.Terminal() {
			return
		}
	}
}

func writeSSE(w http.ResponseWriter, event agent.Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: %s\n", event.Type)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
