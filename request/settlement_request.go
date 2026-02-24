package request

import (
	"fmt"
	"time"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
	"github.com/shopspring/decimal"
)

type FeeItemRequest struct {
	Type        string          `json:"type"`
	Amount      decimal.Decimal `json:"amount"`
	Description string          `json:"description,omitempty"`
}

type SettlementTransactionRequest struct {
	TransactionID      string           `json:"transaction_id"`
	GrossAmount        decimal.Decimal  `json:"gross_amount"`
	NetAmount          decimal.Decimal  `json:"net_amount"`
	Fees               []FeeItemRequest `json:"fees"`
	SettlementDate     time.Time        `json:"settlement_date"`
	ProcessorReference string           `json:"processor_reference,omitempty"`
}

type CreateSettlementRequest struct {
	BatchID           string                         `json:"batch_id"`
	Processor         string                         `json:"processor"`
	SettlementDate    time.Time                      `json:"settlement_date"`
	Currency          models.Currency                `json:"currency"`
	Transactions      []SettlementTransactionRequest `json:"transactions"`
	TotalGross        decimal.Decimal                `json:"total_gross"`
	TotalNet          decimal.Decimal                `json:"total_net"`
	TotalFees         decimal.Decimal                `json:"total_fees"`
	ReserveHoldAmount decimal.Decimal                `json:"reserve_hold_amount"`
	ReportGeneratedAt time.Time                      `json:"report_generated_at"`
	Metadata          map[string]interface{}         `json:"metadata,omitempty"`
}

type BatchSettlementRequest struct {
	Settlements []CreateSettlementRequest `json:"settlements"`
}

func (r *CreateSettlementRequest) Validate() error {
	if r.BatchID == "" {
		return fmt.Errorf("batch_id is required")
	}
	if r.Processor == "" {
		return fmt.Errorf("processor is required")
	}
	if r.SettlementDate.IsZero() {
		return fmt.Errorf("settlement_date is required")
	}
	if !models.ValidCurrencies()[r.Currency] {
		return fmt.Errorf("invalid currency: %s", r.Currency)
	}
	return nil
}

func (r *CreateSettlementRequest) ToModel() models.SettlementReport {
	txns := make([]models.SettlementTransaction, len(r.Transactions))
	for i, t := range r.Transactions {
		fees := make([]models.FeeItem, len(t.Fees))
		for j, f := range t.Fees {
			fees[j] = models.FeeItem{
				Type:        f.Type,
				Amount:      f.Amount,
				Description: f.Description,
			}
		}
		txns[i] = models.SettlementTransaction{
			TransactionID:      t.TransactionID,
			GrossAmount:        t.GrossAmount,
			NetAmount:          t.NetAmount,
			Fees:               fees,
			SettlementDate:     t.SettlementDate,
			ProcessorReference: t.ProcessorReference,
		}
	}

	return models.SettlementReport{
		BatchID:           r.BatchID,
		Processor:         r.Processor,
		SettlementDate:    r.SettlementDate,
		Currency:          r.Currency,
		Transactions:      txns,
		TotalGross:        r.TotalGross,
		TotalNet:          r.TotalNet,
		TotalFees:         r.TotalFees,
		ReserveHoldAmount: r.ReserveHoldAmount,
		ReportGeneratedAt: r.ReportGeneratedAt,
		Metadata:          r.Metadata,
	}
}
