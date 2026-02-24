package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type DiscrepancyType string

const (
	DiscrepancyMissingSettlement             DiscrepancyType = "MISSING_SETTLEMENT"
	DiscrepancyAmountMismatch                DiscrepancyType = "AMOUNT_MISMATCH"
	DiscrepancyDuplicateSettlement           DiscrepancyType = "DUPLICATE_SETTLEMENT"
	DiscrepancyOrphanedSettlement            DiscrepancyType = "ORPHANED_SETTLEMENT"
	DiscrepancyFeeAnomaly                    DiscrepancyType = "FEE_ANOMALY"
	DiscrepancyCurrencyConversion            DiscrepancyType = "CURRENCY_CONVERSION"
	DiscrepancyReserveHold                   DiscrepancyType = "RESERVE_HOLD"
	DiscrepancyChargebackWithoutNotification DiscrepancyType = "CHARGEBACK_WITHOUT_NOTIFICATION"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

type Discrepancy struct {
	ID                   string                 `json:"id"`
	Type                 DiscrepancyType        `json:"type"`
	Severity             Severity               `json:"severity"`
	TransactionID        *string                `json:"transaction_id,omitempty"`
	BatchID              *string                `json:"batch_id,omitempty"`
	ExpectedAmount       *decimal.Decimal       `json:"expected_amount,omitempty"`
	ActualAmount         *decimal.Decimal       `json:"actual_amount,omitempty"`
	Difference           *decimal.Decimal       `json:"difference,omitempty"`
	Currency             *Currency              `json:"currency,omitempty"`
	Processor            *string                `json:"processor,omitempty"`
	Description          string                 `json:"description"`
	InvestigationDetails map[string]interface{} `json:"investigation_details"`
	DetectedAt           time.Time              `json:"detected_at"`
	Resolved             bool                   `json:"resolved"`
}

func AllDiscrepancyTypes() []DiscrepancyType {
	return []DiscrepancyType{
		DiscrepancyMissingSettlement,
		DiscrepancyAmountMismatch,
		DiscrepancyDuplicateSettlement,
		DiscrepancyOrphanedSettlement,
		DiscrepancyFeeAnomaly,
		DiscrepancyCurrencyConversion,
		DiscrepancyReserveHold,
		DiscrepancyChargebackWithoutNotification,
	}
}
