package repository

import (
	"fmt"
	"sync"

	"github.com/akmalkhaniub/recon-mesh/internal/domain/recon"
	"github.com/google/uuid"
)

type Store struct {
	mu     sync.RWMutex
	jobs   map[uuid.UUID]*recon.ReconciliationJob
	breaks map[uuid.UUID]*recon.ReconciliationBreak
}

func NewStore() *Store {
	return &Store{
		jobs:   make(map[uuid.UUID]*recon.ReconciliationJob),
		breaks: make(map[uuid.UUID]*recon.ReconciliationBreak),
	}
}

func (s *Store) SaveJob(job *recon.ReconciliationJob) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.jobs[job.ID] = job
	for i := range job.Breaks {
		b := job.Breaks[i]
		s.breaks[b.ID] = &b
	}
}

func (s *Store) GetJob(id uuid.UUID) (*recon.ReconciliationJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, exists := s.jobs[id]
	if !exists {
		return nil, fmt.Errorf("reconciliation job %s not found", id)
	}
	c := *job
	return &c, nil
}

func (s *Store) ListBreaks(jobID uuid.UUID, breakType *recon.BreakType) ([]recon.ReconciliationBreak, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, exists := s.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("reconciliation job %s not found", jobID)
	}

	var res []recon.ReconciliationBreak
	for _, b := range job.Breaks {
		// fetch latest break state
		currentBreak, ok := s.breaks[b.ID]
		if ok {
			if breakType == nil || currentBreak.Type == *breakType {
				res = append(res, *currentBreak)
			}
		}
	}
	return res, nil
}

func (s *Store) GetBreak(breakID uuid.UUID) (*recon.ReconciliationBreak, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, exists := s.breaks[breakID]
	if !exists {
		return nil, fmt.Errorf("reconciliation break %s not found", breakID)
	}
	return b, nil
}
