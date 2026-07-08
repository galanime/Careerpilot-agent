package scanner_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"careerpilot-agent/internal/domain"
	"careerpilot-agent/internal/opportunities"
	"careerpilot-agent/internal/scanner"
)

func TestCheckLivenessActiveAndClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/closed" {
			w.WriteHeader(http.StatusGone)
			_, _ = w.Write([]byte("job no longer available"))
			return
		}
		_, _ = w.Write([]byte("<h1>AI Agent Engineer</h1><button>Apply</button><p>Requirements</p>"))
	}))
	defer server.Close()

	jobScanner := scanner.New(opportunities.NewStore(filepath.Join(t.TempDir(), "opportunities"), ""))
	active := jobScanner.CheckLiveness(context.Background(), server.URL+"/active")
	if active.Status != domain.LivenessActive {
		t.Fatalf("expected active status, got %#v", active)
	}
	closed := jobScanner.CheckLiveness(context.Background(), server.URL+"/closed")
	if closed.Status != domain.LivenessClosed {
		t.Fatalf("expected closed status, got %#v", closed)
	}
}

func TestDirectScanAddsActiveOpportunity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<h1>Careers</h1><button>Apply</button><p>Job description</p>"))
	}))
	defer server.Close()

	dir := t.TempDir()
	store := opportunities.NewStore(filepath.Join(dir, "opportunities"), filepath.Join(dir, "pipeline.md"))
	jobScanner := scanner.New(store)
	result := jobScanner.Scan(context.Background(), domain.ScanRequest{
		TargetRole: "AI Agent Engineer",
		Companies: []domain.TrackedCompany{{
			Name:       "DemoCorp",
			Provider:   domain.ATSProviderDirect,
			CareersURL: server.URL,
			Enabled:    true,
		}},
	})
	if result.Added != 1 {
		t.Fatalf("expected one added opportunity, got %#v", result)
	}
	opportunities, err := store.List()
	if err != nil {
		t.Fatalf("list opportunities: %v", err)
	}
	if len(opportunities) != 1 || opportunities[0].CompanyName != "DemoCorp" {
		t.Fatalf("unexpected opportunities: %#v", opportunities)
	}
}
