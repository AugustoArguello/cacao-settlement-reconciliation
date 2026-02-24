package request

import (
	"fmt"
	"time"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
	"github.com/shopspring/decimal"
)

type CreateTransactionRequest struct {
	TransactionID       string                 `json:"transaction_id"`
	AuthorizationAmount decimal.Decimal        `json:"authorization_amount"`
	Currency            models.Currency        `json:"currency"`
	Processor           string                 `json:"processor"`
	Timestamp           time.Time              `json:"timestamp"`
	CustomerCountry     string                 `json:"customer_country"`
	PaymentMethod       models.PaymentMethod   `json:"payment_method"`
	Status              models.TransactionStatus `json:"status"`
	MerchantReference   string                 `json:"merchant_reference,omitempty"`
	OriginalCurrency    *models.Currency       `json:"original_currency,omitempty"`
	OriginalAmount      *decimal.Decimal       `json:"original_amount,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

type BatchTransactionRequest struct {
	Transactions []CreateTransactionRequest `json:"transactions"`
}

func (r *CreateTransactionRequest) Validate() error {
	if r.TransactionID == "" {
		return fmt.Errorf("transaction_id is required")
	}
	if r.AuthorizationAmount.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("authorization_amount must be greater than 0")
	}
	if !models.ValidCurrencies()[r.Currency] {
		return fmt.Errorf("invalid currency: %s", r.Currency)
	}
	if r.Processor == "" {
		return fmt.Errorf("processor is required")
	}
	if r.Timestamp.IsZero() {
		return fmt.Errorf("timestamp is required")
	}
	if len(r.CustomerCountry) != 2 {
		return fmt.Errorf("customer_country must be a 2-letter ISO code")
	}
	if !models.ValidPaymentMethods()[r.PaymentMethod] {
		return fmt.Errorf("invalid payment_method: %s", r.PaymentMethod)
	}
	if !models.ValidTransactionStatuses()[r.Status] {
		return fmt.Errorf("invalid status: %s", r.Status)
	}
	return nil
}

func (r *CreateTransactionRequest) ToModel() models.Transaction {
	return models.Transaction{
		TransactionID:       r.TransactionID,
		AuthorizationAmount: r.AuthorizationAmount,
		Currency:            r.Currency,
		Processor:           r.Processor,
		Timestamp:           r.Timestamp,
		CustomerCountry:     r.CustomerCountry,
		PaymentMethod:       r.PaymentMethod,
		Status:              r.Status,
		MerchantReference:   r.MerchantReference,
		OriginalCurrency:    r.OriginalCurrency,
		OriginalAmount:      r.OriginalAmount,
		Metadata:            r.Metadata,
	}
}
