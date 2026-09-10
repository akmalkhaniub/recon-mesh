package ingestion

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/akmalkhaniub/recon-mesh/internal/domain/recon"
	"github.com/google/uuid"
)

// ParseBankStatementCSV ingests standard banking settlement CSV reports
// Format: Date, Reference, Description, AmountInCents, Direction(DEBIT/CREDIT), Currency
func ParseBankStatementCSV(r io.Reader) ([]recon.CanonicalTransaction, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	if len(header) < 6 {
		return nil, fmt.Errorf("invalid CSV header: expected at least 6 columns, got %d", len(header))
	}

	var transactions []recon.CanonicalTransaction

	lineNum := 1
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading line %d: %w", lineNum, err)
		}
		lineNum++

		dateStr := strings.TrimSpace(record[0])
		ref := strings.TrimSpace(record[1])
		amountStr := strings.TrimSpace(record[3])
		directionStr := strings.ToUpper(strings.TrimSpace(record[4]))
		currency := strings.ToUpper(strings.TrimSpace(record[5]))

		timestamp, err := time.Parse(time.RFC3339, dateStr)
		if err != nil {
			// Fallback standard date format
			timestamp, err = time.Parse("2006-01-02", dateStr)
			if err != nil {
				return nil, fmt.Errorf("invalid date format on line %d: %s", lineNum, dateStr)
			}
		}

		amount, err := strconv.ParseInt(amountStr, 10, 64)
		if err != nil || amount <= 0 {
			return nil, fmt.Errorf("invalid positive amount in cents on line %d: %s", lineNum, amountStr)
		}

		var direction recon.TransactionDirection
		if directionStr == "DEBIT" {
			direction = recon.DirectionDebit
		} else if directionStr == "CREDIT" {
			direction = recon.DirectionCredit
		} else {
			return nil, fmt.Errorf("invalid direction on line %d: %s (must be DEBIT or CREDIT)", lineNum, directionStr)
		}

		transactions = append(transactions, recon.CanonicalTransaction{
			ID:        uuid.New(),
			Source:    recon.SourceBank,
			Reference: ref,
			Amount:    amount,
			Currency:  currency,
			Direction: direction,
			Timestamp: timestamp,
		})
	}

	return transactions, nil
}
