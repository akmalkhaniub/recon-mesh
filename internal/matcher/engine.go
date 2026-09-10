package matcher

import (
	"fmt"
	"math"
	"time"

	"github.com/akmalkhaniub/recon-mesh/internal/domain/recon"
	"github.com/google/uuid"
)

// Engine performs deterministic multi-source reconciliation
type Engine struct {
	DateTolerance time.Duration
}

func NewEngine() *Engine {
	return &Engine{
		DateTolerance: 72 * time.Hour, // Standard banking settlement window (ACH/SEPA)
	}
}

// Reconcile processes internal vs external transactions and generates matches and breaks
func (e *Engine) Reconcile(jobID uuid.UUID, currency string, internal []recon.CanonicalTransaction, external []recon.CanonicalTransaction) (*recon.ReconciliationJob, error) {
	start := time.Now()

	matchedInternal := make(map[uuid.UUID]bool)
	matchedExternal := make(map[uuid.UUID]bool)

	var matches []recon.MatchedPair
	var breaks []recon.ReconciliationBreak

	// 1. Build fast lookup index for internal records
	// Index key: reference:direction:amount:currency
	exactIndex := make(map[string][]recon.CanonicalTransaction)
	refIndex := make(map[string][]recon.CanonicalTransaction)

	for _, tx := range internal {
		if tx.Currency != currency {
			continue
		}
		exactKey := fmt.Sprintf("%s:%s:%d:%s", tx.Reference, tx.Direction, tx.Amount, tx.Currency)
		exactIndex[exactKey] = append(exactIndex[exactKey], tx)
		refIndex[tx.Reference] = append(refIndex[tx.Reference], tx)
	}

	// 2. Tier 1: Exact 1-to-1 Match (Reference + Direction + Exact Minor Units)
	for _, ext := range external {
		if ext.Currency != currency {
			continue
		}

		exactKey := fmt.Sprintf("%s:%s:%d:%s", ext.Reference, ext.Direction, ext.Amount, ext.Currency)
		candidates, exists := exactIndex[exactKey]
		if exists && len(candidates) > 0 {
			// Select first unmatched candidate
			for idx, cand := range candidates {
				if !matchedInternal[cand.ID] {
					matchedInternal[cand.ID] = true
					matchedExternal[ext.ID] = true

					matches = append(matches, recon.MatchedPair{
						MatchID:        uuid.New(),
						RuleUsed:       "TIER1_EXACT_REFERENCE_AND_AMOUNT",
						InternalRecord: cand,
						ExternalRecord: ext,
						FeeVariance:    0,
						MatchedAt:      time.Now().UTC(),
					})

					// Remove from candidate pool
					exactIndex[exactKey] = append(candidates[:idx], candidates[idx+1:]...)
					break
				}
			}
		}
	}

	// 3. Tier 2: Windowed Heuristic Match (Same Reference, Same Amount, Date within Tolerance)
	for _, ext := range external {
		if matchedExternal[ext.ID] || ext.Currency != currency {
			continue
		}

		candidates, exists := refIndex[ext.Reference]
		if exists {
			for _, cand := range candidates {
				if !matchedInternal[cand.ID] && cand.Direction == ext.Direction && cand.Amount == ext.Amount {
					diff := cand.Timestamp.Sub(ext.Timestamp)
					if diff < 0 {
						diff = -diff
					}

					if diff <= e.DateTolerance {
						matchedInternal[cand.ID] = true
						matchedExternal[ext.ID] = true

						matches = append(matches, recon.MatchedPair{
							MatchID:        uuid.New(),
							RuleUsed:       "TIER2_WINDOWED_SETTLEMENT_LAG",
							InternalRecord: cand,
							ExternalRecord: ext,
							FeeVariance:    0,
							MatchedAt:      time.Now().UTC(),
						})
						break
					}
				}
			}
		}
	}

	// 4. Tier 3: Identify Amount Mismatches (Same Reference, Different Amounts e.g. Fee deduction)
	for _, ext := range external {
		if matchedExternal[ext.ID] || ext.Currency != currency {
			continue
		}

		candidates, exists := refIndex[ext.Reference]
		if exists {
			for _, cand := range candidates {
				if !matchedInternal[cand.ID] && cand.Direction == ext.Direction && cand.Amount != ext.Amount {
					// Matched reference with amount variance
					matchedInternal[cand.ID] = true
					matchedExternal[ext.ID] = true

					variance := cand.Amount - ext.Amount
					candCopy := cand
					extCopy := ext

					breaks = append(breaks, recon.ReconciliationBreak{
						ID:             uuid.New(),
						JobID:          jobID,
						Type:           recon.BreakAmountMismatch,
						Status:         recon.BreakStatusOpen,
						InternalRecord: &candCopy,
						ExternalRecord: &extCopy,
						VarianceAmount: variance,
						CreatedAt:      time.Now().UTC(),
					})
					break
				}
			}
		}
	}

	// 5. Identify Unmatched Internal Records (Phantom entries / Bank pending)
	for _, in := range internal {
		if !matchedInternal[in.ID] && in.Currency == currency {
			inCopy := in
			breaks = append(breaks, recon.ReconciliationBreak{
				ID:             uuid.New(),
				JobID:          jobID,
				Type:           recon.BreakUnmatchedInternal,
				Status:         recon.BreakStatusOpen,
				InternalRecord: &inCopy,
				VarianceAmount: in.Amount,
				CreatedAt:      time.Now().UTC(),
			})
		}
	}

	// 6. Identify Unmatched External Records (Orphan bank debits / unknown wires)
	for _, ext := range external {
		if !matchedExternal[ext.ID] && ext.Currency == currency {
			extCopy := ext
			breaks = append(breaks, recon.ReconciliationBreak{
				ID:             uuid.New(),
				JobID:          jobID,
				Type:           recon.BreakUnmatchedExternal,
				Status:         recon.BreakStatusOpen,
				ExternalRecord: &extCopy,
				VarianceAmount: ext.Amount,
				CreatedAt:      time.Now().UTC(),
			})
		}
	}

	// 7. Compute Volumes & Statistics
	var matchedVolume int64
	for _, m := range matches {
		matchedVolume += m.InternalRecord.Amount
	}

	var brokenVolume int64
	for _, b := range breaks {
		brokenVolume += int64(math.Abs(float64(b.VarianceAmount)))
	}

	totalRecords := len(internal) + len(external)
	matchedTotal := len(matches) * 2 // each match accounts for 1 internal + 1 external
	matchRate := 0.0
	if totalRecords > 0 {
		matchRate = (float64(matchedTotal) / float64(totalRecords)) * 100.0
	}

	now := time.Now().UTC()
	_ = start // used for metrics

	return &recon.ReconciliationJob{
		ID:                   jobID,
		Status:               "COMPLETED",
		Currency:             currency,
		TotalInternalRecords: len(internal),
		TotalExternalRecords: len(external),
		MatchedCount:         len(matches),
		UnmatchedCount:       len(breaks),
		MatchedVolume:        matchedVolume,
		BrokenVolume:         brokenVolume,
		MatchRatePercentage:  math.Round(matchRate*100) / 100,
		Matches:              matches,
		Breaks:               breaks,
		CreatedAt:            start,
		CompletedAt:          &now,
	}, nil
}
