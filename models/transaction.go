package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type PaymentMethod string

const (
	PaymentMethodCreditCard  PaymentMethod = "credit_card"
	PaymentMethodDebitCard   PaymentMethod = "debit_card"
	PaymentMethodPix         PaymentMethod = "pix"
	PaymentMethodOxxo        PaymentMethod = "oxxo"
	PaymentMethodSPEI        PaymentMethod = "spei"
	PaymentMethodPSE         PaymentMethod = "pse"
	PaymentMethodBankTransfer PaymentMethod = "bank_transfer"
)

type TransactionStatus string

const (
	TransactionStatusAuthorized  TransactionStatus = "AUTHORIZED"
	TransactionStatusCaptured    TransactionStatus = "CAPTURED"
	TransactionStatusSettled     TransactionStatus = "SETTLED"
	TransactionStatusRefunded    TransactionStatus = "REFUNDED"
	TransactionStatusChargedback TransactionStatus = "CHARGEDBACK"
	TransactionStatusDeclined    TransactionStatus = "DECLINED"
)

type Currency string

const (
	CurrencyUSD Currency = "USD"
	CurrencyBRL Currency = "BRL"
	CurrencyEUR Currency = "EUR"
	CurrencyMXN Currency = "MXN"
	CurrencyCOP Currency = "COP"
	CurrencyCLP Currency = "CLP"
)

type Transaction struct {
	TransactionID       string            `json:"transaction_id"`
	AuthorizationAmount decimal.Decimal   `json:"authorization_amount"`
	Currency            Currency          `json:"currency"`
	Processor           string            `json:"processor"`
	Timestamp           time.Time         `json:"timestamp"`
	CustomerCountry     string            `json:"customer_country"`
	PaymentMethod       PaymentMethod     `json:"payment_method"`
	Status              TransactionStatus `json:"status"`
	MerchantReference   string            `json:"merchant_reference,omitempty"`
	OriginalCurrency    *Currency         `json:"original_currency,omitempty"`
	OriginalAmount      *decimal.Decimal  `json:"original_amount,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

func ValidCurrencies() map[Currency]bool {
	return map[Currency]bool{
		CurrencyUSD: true,
		CurrencyBRL: true,
		CurrencyEUR: true,
		CurrencyMXN: true,
		CurrencyCOP: true,
		CurrencyCLP: true,
	}
}

func ValidPaymentMethods() map[PaymentMethod]bool {
	return map[PaymentMethod]bool{
		PaymentMethodCreditCard:   true,
		PaymentMethodDebitCard:    true,
		PaymentMethodPix:          true,
		PaymentMethodOxxo:         true,
		PaymentMethodSPEI:         true,
		PaymentMethodPSE:          true,
		PaymentMethodBankTransfer: true,
	}
}

func ValidTransactionStatuses() map[TransactionStatus]bool {
	return map[TransactionStatus]bool{
		TransactionStatusAuthorized:  true,
		TransactionStatusCaptured:    true,
		TransactionStatusSettled:     true,
		TransactionStatusRefunded:    true,
		TransactionStatusChargedback: true,
		TransactionStatusDeclined:    true,
	}
}
