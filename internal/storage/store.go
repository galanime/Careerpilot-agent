package storage

import (
	"errors"
	"sort"
	"sync"

	"careerpilot-agent/internal/domain"
)

var ErrRunNotFound = errors.New("run not found")

type RunStore interface {
	SaveRun(run domain.Run) error
	GetRun(id string) (domain.Run, error)
	ListRuns() ([]domain.Run, error)
}

type Store struct {
	mu   sync.RWMutex
	runs map[string]domain.Run
}

func NewStore() *Store {
	return &Store{runs: map[string]domain.Run{}}
}

func (s *Store) SaveRun(run domain.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = run
	return nil
}

func (s *Store) GetRun(id string) (domain.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[id]
	if !ok {
		return domain.Run{}, ErrRunNotFound
	}
	return run, nil
}

func (s *Store) ListRuns() ([]domain.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	runs := make([]domain.Run, 0, len(s.runs))
	for _, run := range s.runs {
		runs = append(runs, run)
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})
	return runs, nil
}
