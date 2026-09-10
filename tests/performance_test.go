package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/akmalkhaniub/recon-mesh/internal/domain/recon"
	"github.com/akmalkhaniub/recon-mesh/internal/matcher"
	"github.com/google/uuid"
)

func generateBenchmarkDataset(n int) ([]recon.CanonicalTransaction, []recon.CanonicalTransaction) {
	now := time.Now().UTC()
	internal := make([]recon.CanonicalTransaction, n)
	external := make([]recon.CanonicalTransaction, n)

	for i := 0; i < n; i++ {
		ref := fmt.Sprintf("tx_perf_%d", i)
		amount := int64((i % 50000) + 100) // $1.00 to $500.00

		internal[i] = recon.CanonicalTransaction{
			ID:        uuid.New(),
			Source:    recon.SourceInternal,
			Reference: ref,
			Amount:    amount,
			Currency:  "USD",
			Direction: recon.DirectionCredit,
			Timestamp: now,
		}

		// 98% matching, 2% intentional breaks
		extAmount := amount
		if i >= int(float64(n)*0.98) {
			extAmount += 50 // amount variance
		}

		external[i] = recon.CanonicalTransaction{
			ID:        uuid.New(),
			Source:    recon.SourceBank,
			Reference: ref,
			Amount:    extAmount,
			Currency:  "USD",
			Direction: recon.DirectionCredit,
			Timestamp: now.Add(time.Duration(i%48) * time.Hour),
		}
	}

	return internal, external
}

func TestRecon_HighVolumeThroughput(t *testing.T) {
	const count = 5000 // 5,000 internal + 5,000 external = 10,000 transactions
	internal, external := generateBenchmarkDataset(count)

	engine := matcher.NewEngine()
	start := time.Now()

	jobID := uuid.New()
	job, err := engine.Reconcile(jobID, "USD", internal, external)
	if err != nil {
		t.Fatalf("high volume reconciliation failed: %v", err)
	}

	duration := time.Since(start)
	t.Logf("Reconciled %d transactions in %v (Match rate: %.2f%%, Matches: %d, Breaks: %d)",
		count*2, duration, job.MatchRatePercentage, job.MatchedCount, len(job.Breaks))

	// SLA check: 10,000 transactions must reconcile in under 100 milliseconds
	if duration > 100*time.Millisecond {
		t.Errorf("Performance SLA breached: expected < 100ms, took %v", duration)
	}

	if job.MatchedCount == 0 {
		t.Fatal("expected successful matches")
	}
}

func BenchmarkReconciliationEngine(b *testing.B) {
	internal, external := generateBenchmarkDataset(2000)
	engine := matcher.NewEngine()
	jobID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.Reconcile(jobID, "USD", internal, external)
	}
}
