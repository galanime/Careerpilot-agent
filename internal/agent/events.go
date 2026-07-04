package agent

import (
	"context"
	"sync"
	"time"

	"careerpilot-agent/internal/domain"
)

type EventType string

const (
	EventRunCreated      EventType = "run_created"
	EventRunStarted      EventType = "run_started"
	EventRunFinished     EventType = "run_finished"
	EventRunFailed       EventType = "run_failed"
	EventStepStarted     EventType = "step_started"
	EventStepFinished    EventType = "step_finished"
	EventStepFailed      EventType = "step_failed"
	EventArtifactCreated EventType = "artifact_created"
)

type Event struct {
	ID        string           `json:"id"`
	Type      EventType        `json:"type"`
	RunID     string           `json:"run_id"`
	RunStatus domain.RunStatus `json:"run_status,omitempty"`
	Step      *domain.Step     `json:"step,omitempty"`
	Artifact  *domain.Artifact `json:"artifact,omitempty"`
	Error     string           `json:"error,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
}

func (e Event) Terminal() bool {
	return e.Type == EventRunFinished || e.Type == EventRunFailed
}

type EventHub struct {
	mu      sync.RWMutex
	history map[string][]Event
	subs    map[string]map[chan Event]struct{}
}

func NewEventHub() *EventHub {
	return &EventHub{
		history: map[string][]Event{},
		subs:    map[string]map[chan Event]struct{}{},
	}
}

func (h *EventHub) Publish(event Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.history[event.RunID] = append(h.history[event.RunID], event)
	for ch := range h.subs[event.RunID] {
		select {
		case ch <- event:
		default:
		}
	}
}

func (h *EventHub) Subscribe(ctx context.Context, runID string) <-chan Event {
	ch := make(chan Event, 256)
	h.mu.Lock()
	for _, event := range h.history[runID] {
		ch <- event
	}
	if h.subs[runID] == nil {
		h.subs[runID] = map[chan Event]struct{}{}
	}
	h.subs[runID][ch] = struct{}{}
	h.mu.Unlock()

	go func() {
		<-ctx.Done()
		h.mu.Lock()
		delete(h.subs[runID], ch)
		if len(h.subs[runID]) == 0 {
			delete(h.subs, runID)
		}
		h.mu.Unlock()
		close(ch)
	}()
	return ch
}

func (h *EventHub) History(runID string) []Event {
	h.mu.RLock()
	defer h.mu.RUnlock()
	events := h.history[runID]
	out := make([]Event, len(events))
	copy(out, events)
	return out
}
