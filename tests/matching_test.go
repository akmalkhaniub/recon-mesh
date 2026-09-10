package tests

import (
	"strings"
	"testing"
	"time"

	"github.com/akmalkhaniub/recon-mesh/internal/domain/recon"
	"github.com/akmalkhaniub/recon-mesh/internal/ingestion"
	"github.com/akmalkhaniub/recon-mesh/internal/matcher"
	"github.com/akmalkhaniub/recon-mesh/internal/resolution"
	"github.com/google/uuid"
)

func TestRecon_ExactAndBreakMatching(t *testing.T) {
	engine := matcher.NewEngine()
	now := time.Now().UTC()

	// 1. Matched Pair: Internal ACH credit $150 vs Bank Statement credit $150
	txMatchedInternal := recon.CanonicalTransaction{
		ID:        uuid.New(),
		Source:    recon.SourceInternal,
		Reference: "ach_payout_101",
		Amount:    15000,
		Currency:  "USD",
		Direction: recon.DirectionCredit,
		Timestamp: now,
	}
	txMatchedExternal := recon.CanonicalTransaction{
		ID:        uuid.New(),
		Source:    recon.SourceBank,
		Reference: "ach_payout_101",
		Amount:    15000,
		Currency:  "USD",
		Direction: recon.DirectionCredit,
		Timestamp: now.Add(2 * time.Hour), // Settlement lag within tolerance
	}

	// 2. Amount Mismatch: Internal $500 vs Bank $495 (e.g. $5 bank processing fee deducted)
	txMismatchInternal := recon.CanonicalTransaction{
		ID:        uuid.New(),
		Source:    recon.SourceInternal,
		Reference: "wire_ref_202",
		Amount:    50000,
		Currency:  "USD",
		Direction: recon.DirectionDebit,
		Timestamp: now,
	}
	txMismatchExternal := recon.CanonicalTransaction{
		ID:        uuid.New(),
		Source:    recon.SourceBank,
		Reference: "wire_ref_202",
		Amount:    49500, // $495.00
		Currency:  "USD",
		Direction: recon.DirectionDebit,
		Timestamp: now,
	}

	// 3. Unmatched Internal: Phantom entry posted internally, but never cleared at bank
	txPhantomInternal := recon.CanonicalTransaction{
		ID:        uuid.New(),
		Source:    recon.SourceInternal,
		Reference: "ach_phantom_303",
		Amount:    7500,
		Currency:  "USD",
		Direction: recon.DirectionDebit,
		Timestamp: now,
	}

	// 4. Unmatched External: Unrecorded bank account maintenance fee charged by bank
	txUnrecordedBankFee := recon.CanonicalTransaction{
		ID:        uuid.New(),
		Source:    recon.SourceBank,
		Reference: "bank_monthly_fee_404",
		Amount:    2500,
		Currency:  "USD",
		Direction: recon.DirectionDebit,
		Timestamp: now,
	}

	internal := []recon.CanonicalTransaction{txMatchedInternal, txMismatchInternal, txPhantomInternal}
	external := []recon.CanonicalTransaction{txMatchedExternal, txMismatchExternal, txUnrecordedBankFee}

	jobID := uuid.New()
	job, err := engine.Reconcile(jobID, "USD", internal, external)
	if err != nil {
		t.Fatalf("reconciliation engine failed: %v", err)
	}

	// Assertions
	if job.MatchedCount != 1 {
		t.Errorf("expected 1 exact match, got %d", job.MatchedCount)
	}

	// Verify Breaks: should have 3 breaks (1 amount mismatch, 1 unmatched internal, 1 unmatched external)
	if len(job.Breaks) != 3 {
		t.Fatalf("expected 3 breaks, got %d", len(job.Breaks))
	}

	breakMap := make(map[recon.BreakType]recon.ReconciliationBreak)
	for _, b := range job.Breaks {
		breakMap[b.Type] = b
	}

	// 1. Verify Amount Mismatch Break
	mismatchBreak, ok := breakMap[recon.BreakAmountMismatch]
	if !ok {
		t.Errorf("missing AMOUNT_MISMATCH break")
	} else if mismatchBreak.VarianceAmount != 500 { // 50000 - 49500 = 500 cents ($5.00 fee)
		t.Errorf("expected variance 500 cents, got %d", mismatchBreak.VarianceAmount)
	}

	// 2. Verify Unmatched Internal Break
	unmatchedInt, ok := breakMap[recon.BreakUnmatchedInternal]
	if !ok {
		t.Errorf("missing UNMATCHED_INTERNAL break")
	} else if unmatchedInt.InternalRecord.Reference != "ach_phantom_303" {
		t.Errorf("unexpected reference for unmatched internal: %s", unmatchedInt.InternalRecord.Reference)
	}

	// 3. Verify Unmatched External Break
	unmatchedExt, ok := breakMap[recon.BreakUnmatchedExternal]
	if !ok {
		t.Errorf("missing UNMATCHED_EXTERNAL break")
	} else if unmatchedExt.ExternalRecord.Reference != "bank_monthly_fee_404" {
		t.Errorf("unexpected reference for unmatched external: %s", unmatchedExt.ExternalRecord.Reference)
	}

	// Test Break Resolution: Post Suspense Entry
	resolver := resolution.NewManager()
	entry, err := resolver.ResolveBreak(&mismatchBreak, recon.ActionPostSuspense, "Posting $5 fee to suspense pending bank fee invoice", "ops_lead_01")
	if err != nil {
		t.Fatalf("failed to resolve break: %v", err)
	}

	if mismatchBreak.Status != recon.BreakStatusResolved {
		t.Errorf("expected break status RESOLVED, got %s", mismatchBreak.Status)
	}
	if entry.Amount != 500 {
		t.Errorf("expected suspense entry amount 500, got %d", entry.Amount)
	}
}

func TestRecon_CSVIngestionParser(t *testing.T) {
	csvData := `Date,Reference,Description,AmountInCents,Direction,Currency
2026-09-08T10:00:00Z,col_ach_001,Customer Payout,12500,DEBIT,USD
2026-09-08T12:30:00Z,col_ach_002,Merchant Deposit,89000,CREDIT,USD
`
	transactions, err := ingestion.ParseBankStatementCSV(strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("failed to parse bank CSV: %v", err)
	}

	if len(transactions) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(transactions))
	}

	if transactions[0].Reference != "col_ach_001" || transactions[0].Amount != 12500 {
		t.Errorf("unexpected parsed transaction data: %+v", transactions[0])
	}
	if transactions[1].Direction != recon.DirectionCredit || transactions[1].Amount != 89000 {
		t.Errorf("unexpected second transaction data: %+v", transactions[1])
	}
}
