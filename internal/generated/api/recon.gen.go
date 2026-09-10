// Package api provides primitives to interact with the ReconMesh openapi HTTP API.
// Code generated from api/openapi/v1/recon.yaml DO NOT EDIT.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
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
	ActionManualMatch     ResolutionAction = "MANUAL_MATCH"
	ActionWriteOff        ResolutionAction = "WRITE_OFF"
	ActionPostSuspense    ResolutionAction = "POST_SUSPENSE_ENTRY"
)

type CanonicalTransaction struct {
	Id        uuid.UUID            `json:"id"`
	Source    RecordSource         `json:"source"`
	Reference string               `json:"reference"`
	Amount    int64                `json:"amount"` // Minor units (cents)
	Currency  string               `json:"currency"`
	Direction TransactionDirection `json:"direction"`
	Timestamp time.Time            `json:"timestamp"`
}

type CreateReconciliationJobRequest struct {
	Currency        string                 `json:"currency"`
	InternalRecords []CanonicalTransaction `json:"internal_records"`
	ExternalRecords []CanonicalTransaction `json:"external_records"`
}

type ReconciliationJob struct {
	Id                   uuid.UUID `json:"id"`
	Status               string    `json:"status"`
	Currency             string    `json:"currency"`
	TotalInternalRecords int       `json:"total_internal_records"`
	TotalExternalRecords int       `json:"total_external_records"`
	MatchedCount         int       `json:"matched_count"`
	UnmatchedCount       int       `json:"unmatched_count"`
	MatchedVolume        int64     `json:"matched_volume"`
	BrokenVolume         int64     `json:"broken_volume"`
	MatchRatePercentage  float64   `json:"match_rate_percentage"`
	CreatedAt            time.Time `json:"created_at"`
}

type ReconciliationBreak struct {
	Id              uuid.UUID             `json:"id"`
	JobId           uuid.UUID             `json:"job_id"`
	Type            BreakType             `json:"type"`
	Status          BreakStatus           `json:"status"`
	InternalRecord  *CanonicalTransaction `json:"internal_record,omitempty"`
	ExternalRecord  *CanonicalTransaction `json:"external_record,omitempty"`
	VarianceAmount  int64                 `json:"variance_amount"`
	SuspenseEntryId *uuid.UUID            `json:"suspense_entry_id,omitempty"`
	ResolutionNote  *string               `json:"resolution_note,omitempty"`
	ResolvedBy      *string               `json:"resolved_by,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
}

type BreakList struct {
	Items []ReconciliationBreak `json:"items"`
	Total int                   `json:"total"`
}

type ResolveBreakRequest struct {
	Action         ResolutionAction `json:"action"`
	ResolutionNote string           `json:"resolution_note"`
	ResolvedBy     string           `json:"resolved_by"`
}

type ReconciliationReport struct {
	JobId               uuid.UUID `json:"job_id"`
	ExecutionTimeMs     int64     `json:"execution_time_ms"`
	Currency            string    `json:"currency"`
	MatchRate           float64   `json:"match_rate"`
	TotalInternalVolume int64     `json:"total_internal_volume"`
	TotalExternalVolume int64     `json:"total_external_volume"`
	NetVariance         int64     `json:"net_variance"`
	OpenBreaksCount     int       `json:"open_breaks_count"`
}

type ProblemDetails struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
	Status int    `json:"status"`
	Title  string `json:"title"`
	Type   string `json:"type"`
}

type ServerInterface interface {
	CreateReconciliationJob(w http.ResponseWriter, r *http.Request)
	GetReconciliationJob(w http.ResponseWriter, r *http.Request, jobId uuid.UUID)
	ListReconciliationBreaks(w http.ResponseWriter, r *http.Request, jobId uuid.UUID)
	ResolveBreak(w http.ResponseWriter, r *http.Request, breakId uuid.UUID)
	GetReconciliationReport(w http.ResponseWriter, r *http.Request, jobId uuid.UUID)
}

func HandlerFromMux(si ServerInterface, r chi.Router) http.Handler {
	r.Group(func(r chi.Router) {
		r.Post("/v1/reconciliation/jobs", si.CreateReconciliationJob)
		r.Get("/v1/reconciliation/jobs/{jobId}", func(w http.ResponseWriter, r *http.Request) {
			id, err := uuid.Parse(chi.URLParam(r, "jobId"))
			if err != nil {
				RespondProblem(w, http.StatusBadRequest, "INVALID_UUID", "Invalid jobId format")
				return
			}
			si.GetReconciliationJob(w, r, id)
		})
		r.Get("/v1/reconciliation/jobs/{jobId}/breaks", func(w http.ResponseWriter, r *http.Request) {
			id, err := uuid.Parse(chi.URLParam(r, "jobId"))
			if err != nil {
				RespondProblem(w, http.StatusBadRequest, "INVALID_UUID", "Invalid jobId format")
				return
			}
			si.ListReconciliationBreaks(w, r, id)
		})
		r.Post("/v1/reconciliation/breaks/{breakId}/resolve", func(w http.ResponseWriter, r *http.Request) {
			id, err := uuid.Parse(chi.URLParam(r, "breakId"))
			if err != nil {
				RespondProblem(w, http.StatusBadRequest, "INVALID_UUID", "Invalid breakId format")
				return
			}
			si.ResolveBreak(w, r, id)
		})
		r.Get("/v1/reconciliation/jobs/{jobId}/report", func(w http.ResponseWriter, r *http.Request) {
			id, err := uuid.Parse(chi.URLParam(r, "jobId"))
			if err != nil {
				RespondProblem(w, http.StatusBadRequest, "INVALID_UUID", "Invalid jobId format")
				return
			}
			si.GetReconciliationReport(w, r, id)
		})
	})
	return r
}

func RespondProblem(w http.ResponseWriter, status int, code string, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ProblemDetails{
		Type:   fmt.Sprintf("https://api.reconmesh.io/errors/%s", code),
		Title:  http.StatusText(status),
		Status: status,
		Code:   code,
		Detail: detail,
	})
}
