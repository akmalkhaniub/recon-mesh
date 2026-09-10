package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/akmalkhaniub/recon-mesh/internal/api"
	gen "github.com/akmalkhaniub/recon-mesh/internal/generated/api"
	"github.com/akmalkhaniub/recon-mesh/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func setupTestServer() http.Handler {
	store := repository.NewStore()
	apiHandler := api.NewReconciliationAPI(store)

	r := chi.NewRouter()
	gen.HandlerFromMux(apiHandler, r)
	return r
}

func TestE2E_ReconciliationLifecycle(t *testing.T) {
	handler := setupTestServer()
	now := time.Now().UTC()

	// 1. Trigger Reconciliation Job via POST /v1/reconciliation/jobs
	reqPayload := gen.CreateReconciliationJobRequest{
		Currency: "USD",
		InternalRecords: []gen.CanonicalTransaction{
			{
				Id:        uuid.New(),
				Source:    gen.SourceInternal,
				Reference: "ref_tx_901",
				Amount:    35000, // $350.00
				Currency:  "USD",
				Direction: gen.DirectionDebit,
				Timestamp: now,
			},
			{
				Id:        uuid.New(),
				Source:    gen.SourceInternal,
				Reference: "ref_tx_phantom_902",
				Amount:    12000, // $120.00
				Currency:  "USD",
				Direction: gen.DirectionDebit,
				Timestamp: now,
			},
		},
		ExternalRecords: []gen.CanonicalTransaction{
			{
				Id:        uuid.New(),
				Source:    gen.SourceBank,
				Reference: "ref_tx_901",
				Amount:    35000,
				Currency:  "USD",
				Direction: gen.DirectionDebit,
				Timestamp: now.Add(time.Hour),
			},
		},
	}

	bodyBytes, _ := json.Marshal(reqPayload)
	req, _ := http.NewRequest(http.MethodPost, "/v1/reconciliation/jobs", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var job gen.ReconciliationJob
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if job.MatchedCount != 1 || job.UnmatchedCount != 1 {
		t.Errorf("expected 1 match and 1 break, got %d matches and %d breaks", job.MatchedCount, job.UnmatchedCount)
	}

	// 2. Fetch Breaks via GET /v1/reconciliation/jobs/{id}/breaks
	breaksReq, _ := http.NewRequest(http.MethodGet, "/v1/reconciliation/jobs/"+job.Id.String()+"/breaks", nil)
	breaksRec := httptest.NewRecorder()
	handler.ServeHTTP(breaksRec, breaksReq)

	if breaksRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for breaks, got %d", breaksRec.Code)
	}

	var breakList gen.BreakList
	_ = json.Unmarshal(breaksRec.Body.Bytes(), &breakList)

	if breakList.Total != 1 {
		t.Fatalf("expected 1 break in list, got %d", breakList.Total)
	}

	breakItem := breakList.Items[0]
	if breakItem.Type != gen.BreakUnmatchedInternal {
		t.Errorf("expected UNMATCHED_INTERNAL break type, got %s", breakItem.Type)
	}

	// 3. Resolve Break via POST /v1/reconciliation/breaks/{id}/resolve
	resolveBody := gen.ResolveBreakRequest{
		Action:         gen.ActionPostSuspense,
		ResolutionNote: "Bank ACH delay verified, moving to discrepancy reserve",
		ResolvedBy:     "ops_agent_audit",
	}
	resBytes, _ := json.Marshal(resolveBody)

	resolveReq, _ := http.NewRequest(http.MethodPost, "/v1/reconciliation/breaks/"+breakItem.Id.String()+"/resolve", bytes.NewBuffer(resBytes))
	resolveReq.Header.Set("Content-Type", "application/json")

	resolveRec := httptest.NewRecorder()
	handler.ServeHTTP(resolveRec, resolveReq)

	if resolveRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for resolve break, got %d: %s", resolveRec.Code, resolveRec.Body.String())
	}

	var resolvedBreak gen.ReconciliationBreak
	_ = json.Unmarshal(resolveRec.Body.Bytes(), &resolvedBreak)
	if resolvedBreak.Status != gen.BreakStatusResolved {
		t.Errorf("expected break status RESOLVED, got %s", resolvedBreak.Status)
	}

	// 4. Download EOD Audit Report via GET /v1/reconciliation/jobs/{id}/report
	repReq, _ := http.NewRequest(http.MethodGet, "/v1/reconciliation/jobs/"+job.Id.String()+"/report", nil)
	repRec := httptest.NewRecorder()
	handler.ServeHTTP(repRec, repReq)

	if repRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for report, got %d", repRec.Code)
	}

	var report gen.ReconciliationReport
	_ = json.Unmarshal(repRec.Body.Bytes(), &report)
	if report.OpenBreaksCount != 0 {
		t.Errorf("expected 0 open breaks after resolution, got %d", report.OpenBreaksCount)
	}
}
