package recon

import (
	"time"

	"github.com/google/uuid"
)

type RecordSource string

const (
	SourceInternal  RecordSource = "INTERNAL_LEDGER"
	SourceBank      RecordSource = "BANK_STATEMENT"
	SourceProcessor RecordSource = "PROCESSOR_GATEWAY"
)

type TransactionDirection string

const (
	DirectionDebit  TransactionDirection = "DEBIT"
	DirectionCredit TransactionDirection = "CREDIT"
)

type BreakType string

const (
	BreakUnmatchedInternal BreakType = "UNMATCHED_INTERNAL"
	BreakUnmatchedExternal BreakType = "UNMATCHED_EXTERNAL"
	BreakAmountMismatch    BreakType = "AMOUNT_MISMATCH"
	BreakDateOutOfBounds   BreakType = "DATE_OUT_OF_BOUNDS"
)

type BreakStatus string

const (
	BreakStatusOpen      BreakStatus = "OPEN"
	BreakStatusResolved  BreakStatus = "RESOLVED"
	BreakStatusEscalated BreakStatus = "ESCALATED"
)

type ResolutionAction string

const (
	ActionManualMatch  ResolutionAction = "MANUAL_MATCH"
	ActionWriteOff     ResolutionAction = "WRITE_OFF"
	ActionPostSuspense ResolutionAction = "POST_SUSPENSE_ENTRY"
)

// CanonicalTransaction represents a normalized financial event across all sources
type CanonicalTransaction struct {
	ID        uuid.UUID            `json:"id"`
	Source    RecordSource         `json:"source"`
	Reference string               `json:"reference"`
	Amount    int64                `json:"amount"` // Minor currency units (e.g. cents)
	Currency  string               `json:"currency"`
	Direction TransactionDirection `json:"direction"`
	Timestamp time.Time            `json:"timestamp"`
}

// MatchedPair represents a successfully reconciled set of financial records
type MatchedPair struct {
	MatchID        uuid.UUID             `json:"match_id"`
	RuleUsed       string                `json:"rule_used"`
	InternalRecord CanonicalTransaction  `json:"internal_record"`
	ExternalRecord CanonicalTransaction  `json:"external_record"`
	FeeVariance    int64                 `json:"fee_variance"`
	MatchedAt      time.Time             `json:"matched_at"`
}

// ReconciliationBreak represents an exception or discrepancy requiring investigation
type ReconciliationBreak struct {
	ID              uuid.UUID             `json:"id"`
	JobID           uuid.UUID             `json:"job_id"`
	Type            BreakType             `json:"type"`
	Status          BreakStatus           `json:"status"`
	InternalRecord  *CanonicalTransaction `json:"internal_record,omitempty"`
	ExternalRecord  *CanonicalTransaction `json:"external_record,omitempty"`
	VarianceAmount  int64                 `json:"variance_amount"`
	SuspenseEntryID *uuid.UUID            `json:"suspense_entry_id,omitempty"`
	ResolutionNote  *string               `json:"resolution_note,omitempty"`
	ResolvedBy      *string               `json:"resolved_by,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
}

// ReconciliationJob represents an execution run of the reconciliation pipeline
type ReconciliationJob struct {
	ID                   uuid.UUID             `json:"id"`
	Status               string                `json:"status"`
	Currency             string                `json:"currency"`
	TotalInternalRecords int                   `json:"total_internal_records"`
	TotalExternalRecords int                   `json:"total_external_records"`
	MatchedCount         int                   `json:"matched_count"`
	UnmatchedCount       int                   `json:"unmatched_count"`
	MatchedVolume        int64                 `json:"matched_volume"`
	BrokenVolume         int64                 `json:"broken_volume"`
	MatchRatePercentage  float64               `json:"match_rate_percentage"`
	Matches              []MatchedPair         `json:"-"`
	Breaks               []ReconciliationBreak `json:"-"`
	CreatedAt            time.Time             `json:"created_at"`
	CompletedAt          *time.Time            `json:"completed_at,omitempty"`
}

// ReconciliationReport summarizes EOD financial integrity
type ReconciliationReport struct {
	JobID               uuid.UUID `json:"job_id"`
	ExecutionTimeMs     int64     `json:"execution_time_ms"`
	Currency            string    `json:"currency"`
	MatchRate           float64   `json:"match_rate"`
	TotalInternalVolume int64     `json:"total_internal_volume"`
	TotalExternalVolume int64     `json:"total_external_volume"`
	NetVariance         int64     `json:"net_variance"`
	OpenBreaksCount     int       `json:"open_breaks_count"`
}
