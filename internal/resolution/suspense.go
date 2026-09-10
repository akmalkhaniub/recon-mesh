package resolution

import (
	"errors"
	"fmt"
	"time"

	"github.com/akmalkhaniub/recon-mesh/internal/domain/recon"
	"github.com/google/uuid"
)

var (
	ErrBreakNotFound = errors.New("reconciliation break not found")
	ErrAlreadyResolved = errors.New("reconciliation break has already been resolved")
)

// SuspenseJournalEntry represents a balancing entry posted to the financial ledger
type SuspenseJournalEntry struct {
	ID          uuid.UUID
	BreakID     uuid.UUID
	Account     string // e.g. "Assets:ReconciliationSuspense" or "Expense:WriteOff"
	Direction   string
	Amount      int64
	Currency    string
	Description string
	CreatedAt   time.Time
}

// Manager handles break resolution workflows and suspense journal entries
type Manager struct {
	suspenseEntries []SuspenseJournalEntry
}

func NewManager() *Manager {
	return &Manager{
		suspenseEntries: make([]SuspenseJournalEntry, 0),
	}
}

// ResolveBreak applies an audited resolution action to an open reconciliation break
func (m *Manager) ResolveBreak(b *recon.ReconciliationBreak, action recon.ResolutionAction, note string, resolvedBy string) (*SuspenseJournalEntry, error) {
	if b.Status == recon.BreakStatusResolved {
		return nil, ErrAlreadyResolved
	}

	now := time.Now().UTC()
	b.Status = recon.BreakStatusResolved
	b.ResolutionNote = &note
	b.ResolvedBy = &resolvedBy

	var entry *SuspenseJournalEntry

	switch action {
	case recon.ActionPostSuspense:
		// Generate offsetting suspense journal entry
		account := "Assets:ReconciliationSuspense"
		direction := "DEBIT"
		if b.Type == recon.BreakUnmatchedInternal {
			account = "Liabilities:DiscrepancyReserve"
			direction = "CREDIT"
		}

		entryID := uuid.New()
		b.SuspenseEntryID = &entryID

		entry = &SuspenseJournalEntry{
			ID:          entryID,
			BreakID:     b.ID,
			Account:     account,
			Direction:   direction,
			Amount:      b.VarianceAmount,
			Currency:    "USD",
			Description: fmt.Sprintf("Suspense rebalance for break %s: %s", b.ID, note),
			CreatedAt:   now,
		}
		m.suspenseEntries = append(m.suspenseEntries, *entry)

	case recon.ActionWriteOff:
		entryID := uuid.New()
		b.SuspenseEntryID = &entryID

		entry = &SuspenseJournalEntry{
			ID:          entryID,
			BreakID:     b.ID,
			Account:     "Expense:ReconciliationBreakage",
			Direction:   "DEBIT",
			Amount:      b.VarianceAmount,
			Currency:    "USD",
			Description: fmt.Sprintf("Write-off for break %s: %s", b.ID, note),
			CreatedAt:   now,
		}
		m.suspenseEntries = append(m.suspenseEntries, *entry)

	case recon.ActionManualMatch:
		// No financial journal entry needed for manual match, audited via note
	}

	return entry, nil
}
