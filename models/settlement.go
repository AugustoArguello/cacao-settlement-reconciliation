package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type FeeItem struct {
	Type        string          `json:"type"`
	Amount      decimal.Decimal `json:"amount"`
	Description string          `json:"description,omitempty"`
}

type SettlementTransaction struct {
	TransactionID      string          `json:"transaction_id"`
	GrossAmount        decimal.Decimal `json:"gross_amount"`
	NetAmount          decimal.Decimal `json:"net_amount"`
	Fees               []FeeItem       `json:"fees"`
	SettlementDate     time.Time       `json:"settlement_date"`
	ProcessorReference string          `json:"processor_reference,omitempty"`
}

type SettlementReport struct {
	BatchID           string                  `json:"batch_id"`
	Processor         string                  `json:"processor"`
	SettlementDate    time.Time               `json:"settlement_date"`
	Currency          Currency                `json:"currency"`
	Transactions      []SettlementTransaction `json:"transactions"`
	TotalGross        decimal.Decimal         `json:"total_gross"`
	TotalNet          decimal.Decimal         `json:"total_net"`
	TotalFees         decimal.Decimal         `json:"total_fees"`
	ReserveHoldAmount decimal.Decimal         `json:"reserve_hold_amount"`
	ReportGeneratedAt time.Time               `json:"report_generated_at"`
	Metadata          map[string]interface{}  `json:"metadata,omitempty"`
}
