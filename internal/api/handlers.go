package api

import (
	"encoding/json"
	"net/http"

	"github.com/akmalkhaniub/recon-mesh/internal/domain/recon"
	gen "github.com/akmalkhaniub/recon-mesh/internal/generated/api"
	"github.com/akmalkhaniub/recon-mesh/internal/matcher"
	"github.com/akmalkhaniub/recon-mesh/internal/repository"
	"github.com/akmalkhaniub/recon-mesh/internal/resolution"
	"github.com/google/uuid"
)

type ReconciliationAPI struct {
	engine   *matcher.Engine
	store    *repository.Store
	resolver *resolution.Manager
}

func NewReconciliationAPI(store *repository.Store) *ReconciliationAPI {
	return &ReconciliationAPI{
		engine:   matcher.NewEngine(),
		store:    store,
		resolver: resolution.NewManager(),
	}
}

// CreateReconciliationJob handles POST /v1/reconciliation/jobs
func (a *ReconciliationAPI) CreateReconciliationJob(w http.ResponseWriter, r *http.Request) {
	var req gen.CreateReconciliationJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		gen.RespondProblem(w, http.StatusBadRequest, "MALFORMED_JSON", "Invalid JSON payload")
		return
	}

	if req.Currency == "" {
		gen.RespondProblem(w, http.StatusBadRequest, "VALIDATION_FAILED", "currency is required")
		return
	}

	internalTx := make([]recon.CanonicalTransaction, len(req.InternalRecords))
	for i, t := range req.InternalRecords {
		id := t.Id
		if id == uuid.Nil {
			id = uuid.New()
		}
		internalTx[i] = recon.CanonicalTransaction{
			ID:        id,
			Source:    recon.RecordSource(t.Source),
			Reference: t.Reference,
			Amount:    t.Amount,
			Currency:  t.Currency,
			Direction: recon.TransactionDirection(t.Direction),
			Timestamp: t.Timestamp,
		}
	}

	externalTx := make([]recon.CanonicalTransaction, len(req.ExternalRecords))
	for i, t := range req.ExternalRecords {
		id := t.Id
		if id == uuid.Nil {
			id = uuid.New()
		}
		externalTx[i] = recon.CanonicalTransaction{
			ID:        id,
			Source:    recon.RecordSource(t.Source),
			Reference: t.Reference,
			Amount:    t.Amount,
			Currency:  t.Currency,
			Direction: recon.TransactionDirection(t.Direction),
			Timestamp: t.Timestamp,
		}
	}

	jobID := uuid.New()
	job, err := a.engine.Reconcile(jobID, req.Currency, internalTx, externalTx)
	if err != nil {
		gen.RespondProblem(w, http.StatusInternalServerError, "RECON_ENGINE_ERROR", err.Error())
		return
	}

	a.store.SaveJob(job)

	res := gen.ReconciliationJob{
		Id:                   job.ID,
		Status:               job.Status,
		Currency:             job.Currency,
		TotalInternalRecords: job.TotalInternalRecords,
		TotalExternalRecords: job.TotalExternalRecords,
		MatchedCount:         job.MatchedCount,
		UnmatchedCount:       job.UnmatchedCount,
		MatchedVolume:        job.MatchedVolume,
		BrokenVolume:         job.BrokenVolume,
		MatchRatePercentage:  job.MatchRatePercentage,
		CreatedAt:            job.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(res)
}

// GetReconciliationJob handles GET /v1/reconciliation/jobs/{jobId}
func (a *ReconciliationAPI) GetReconciliationJob(w http.ResponseWriter, r *http.Request, jobId uuid.UUID) {
	job, err := a.store.GetJob(jobId)
	if err != nil {
		gen.RespondProblem(w, http.StatusNotFound, "JOB_NOT_FOUND", "Reconciliation job not found")
		return
	}

	res := gen.ReconciliationJob{
		Id:                   job.ID,
		Status:               job.Status,
		Currency:             job.Currency,
		TotalInternalRecords: job.TotalInternalRecords,
		TotalExternalRecords: job.TotalExternalRecords,
		MatchedCount:         job.MatchedCount,
		UnmatchedCount:       job.UnmatchedCount,
		MatchedVolume:        job.MatchedVolume,
		BrokenVolume:         job.BrokenVolume,
		MatchRatePercentage:  job.MatchRatePercentage,
		CreatedAt:            job.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// ListReconciliationBreaks handles GET /v1/reconciliation/jobs/{jobId}/breaks
func (a *ReconciliationAPI) ListReconciliationBreaks(w http.ResponseWriter, r *http.Request, jobId uuid.UUID) {
	breakTypeParam := r.URL.Query().Get("type")
	var filterType *recon.BreakType
	if breakTypeParam != "" {
		bt := recon.BreakType(breakTypeParam)
		filterType = &bt
	}

	breaks, err := a.store.ListBreaks(jobId, filterType)
	if err != nil {
		gen.RespondProblem(w, http.StatusNotFound, "JOB_NOT_FOUND", "Job not found")
		return
	}

	items := make([]gen.ReconciliationBreak, len(breaks))
	for i, b := range breaks {
		var inRec *gen.CanonicalTransaction
		if b.InternalRecord != nil {
			inRec = &gen.CanonicalTransaction{
				Id:        b.InternalRecord.ID,
				Source:    gen.RecordSource(b.InternalRecord.Source),
				Reference: b.InternalRecord.Reference,
				Amount:    b.InternalRecord.Amount,
				Currency:  b.InternalRecord.Currency,
				Direction: gen.TransactionDirection(b.InternalRecord.Direction),
				Timestamp: b.InternalRecord.Timestamp,
			}
		}

		var extRec *gen.CanonicalTransaction
		if b.ExternalRecord != nil {
			extRec = &gen.CanonicalTransaction{
				Id:        b.ExternalRecord.ID,
				Source:    gen.RecordSource(b.ExternalRecord.Source),
				Reference: b.ExternalRecord.Reference,
				Amount:    b.ExternalRecord.Amount,
				Currency:  b.ExternalRecord.Currency,
				Direction: gen.TransactionDirection(b.ExternalRecord.Direction),
				Timestamp: b.ExternalRecord.Timestamp,
			}
		}

		items[i] = gen.ReconciliationBreak{
			Id:              b.ID,
			JobId:           b.JobID,
			Type:            gen.BreakType(b.Type),
			Status:          gen.BreakStatus(b.Status),
			InternalRecord:  inRec,
			ExternalRecord:  extRec,
			VarianceAmount:  b.VarianceAmount,
			SuspenseEntryId: b.SuspenseEntryID,
			ResolutionNote:  b.ResolutionNote,
			ResolvedBy:      b.ResolvedBy,
			CreatedAt:       b.CreatedAt,
		}
	}

	res := gen.BreakList{
		Items: items,
		Total: len(items),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// ResolveBreak handles POST /v1/reconciliation/breaks/{breakId}/resolve
func (a *ReconciliationAPI) ResolveBreak(w http.ResponseWriter, r *http.Request, breakId uuid.UUID) {
	var req gen.ResolveBreakRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		gen.RespondProblem(w, http.StatusBadRequest, "MALFORMED_JSON", "Invalid resolution payload")
		return
	}

	b, err := a.store.GetBreak(breakId)
	if err != nil {
		gen.RespondProblem(w, http.StatusNotFound, "BREAK_NOT_FOUND", "Reconciliation break not found")
		return
	}

	_, err = a.resolver.ResolveBreak(b, recon.ResolutionAction(req.Action), req.ResolutionNote, req.ResolvedBy)
	if err != nil {
		gen.RespondProblem(w, http.StatusConflict, "RESOLUTION_CONFLICT", err.Error())
		return
	}

	res := gen.ReconciliationBreak{
		Id:              b.ID,
		JobId:           b.JobID,
		Type:            gen.BreakType(b.Type),
		Status:          gen.BreakStatus(b.Status),
		VarianceAmount:  b.VarianceAmount,
		SuspenseEntryId: b.SuspenseEntryID,
		ResolutionNote:  b.ResolutionNote,
		ResolvedBy:      b.ResolvedBy,
		CreatedAt:       b.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// GetReconciliationReport handles GET /v1/reconciliation/jobs/{jobId}/report
func (a *ReconciliationAPI) GetReconciliationReport(w http.ResponseWriter, r *http.Request, jobId uuid.UUID) {
	job, err := a.store.GetJob(jobId)
	if err != nil {
		gen.RespondProblem(w, http.StatusNotFound, "JOB_NOT_FOUND", "Reconciliation job not found")
		return
	}

	breaks, _ := a.store.ListBreaks(jobId, nil)
	var openBreaks int
	for _, b := range breaks {
		if b.Status == recon.BreakStatusOpen {
			openBreaks++
		}
	}

	duration := int64(0)
	if job.CompletedAt != nil {
		duration = job.CompletedAt.Sub(job.CreatedAt).Milliseconds()
	}

	report := gen.ReconciliationReport{
		JobId:               job.ID,
		ExecutionTimeMs:     duration,
		Currency:            job.Currency,
		MatchRate:           job.MatchRatePercentage,
		TotalInternalVolume: job.MatchedVolume,
		TotalExternalVolume: job.MatchedVolume,
		NetVariance:         job.BrokenVolume,
		OpenBreaksCount:     openBreaks,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}
