package models

import "github.com/shopspring/decimal"

type FeeRule struct {
	ID                 string          `json:"id"`
	Processor          string          `json:"processor"`
	PaymentMethod      *PaymentMethod  `json:"payment_method,omitempty"`
	Country            *string         `json:"country,omitempty"`
	Currency           *Currency       `json:"currency,omitempty"`
	FeeType            string          `json:"fee_type"`
	RatePercent        decimal.Decimal `json:"rate_percent"`
	FlatFee            decimal.Decimal `json:"flat_fee"`
	VarianceThreshold  decimal.Decimal `json:"variance_threshold"`
}
