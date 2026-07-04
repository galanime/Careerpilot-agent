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
	"careerpilot-agent/internal/storage"
)

type Server struct {
	runtime *agent.Runtime
}

func NewServer(runtime *agent.Runtime) *Server {
	return &Server{runtime: runtime}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/runs", s.handleListRuns)
	mux.HandleFunc("POST /api/runs", s.handleCreateRun)
	mux.HandleFunc("GET /api/runs/", s.handleRunSubresource)
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
