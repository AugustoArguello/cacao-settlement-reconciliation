package tests

import (
	"net/http"
	"testing"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/request"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/response"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"encoding/json"
	"time"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
)

// =====================================================
// Transaction Validation Tests
// =====================================================

func TestCreateTransaction_InvalidJSON(t *testing.T) {
	e, _ := setupTestServer()
	rec := doRequest(e, http.MethodPost, "/api/v1/transactions", "{ invalid json }")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp response.ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &errResp)
	assert.Equal(t, "INVALID_JSON", errResp.Error.Code)
}

func TestCreateTransaction_MissingTransactionID(t *testing.T) {
	e, _ := setupTestServer()

	req := request.CreateTransactionRequest{
		TransactionID:       "", // empty
		AuthorizationAmount: decimal.NewFromFloat(100),
		Currency:            models.CurrencyUSD,
		Processor:           "stripe",
		Timestamp:           time.Now(),
		CustomerCountry:     "US",
		PaymentMethod:       models.PaymentMethodCreditCard,
		Status:              models.TransactionStatusCaptured,
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/transactions", req)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestCreateTransaction_NegativeAmount(t *testing.T) {
	e, _ := setupTestServer()

	req := request.CreateTransactionRequest{
		TransactionID:       "txn_neg",
		AuthorizationAmount: decimal.NewFromFloat(-500),
		Currency:            models.CurrencyUSD,
		Processor:           "stripe",
		Timestamp:           time.Now(),
		CustomerCountry:     "US",
		PaymentMethod:       models.PaymentMethodCreditCard,
		Status:              models.TransactionStatusCaptured,
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/transactions", req)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestCreateTransaction_ZeroAmount(t *testing.T) {
	e, _ := setupTestServer()

	req := request.CreateTransactionRequest{
		TransactionID:       "txn_zero",
		AuthorizationAmount: decimal.NewFromFloat(0),
		Currency:            models.CurrencyUSD,
		Processor:           "stripe",
		Timestamp:           time.Now(),
		CustomerCountry:     "US",
		PaymentMethod:       models.PaymentMethodCreditCard,
		Status:              models.TransactionStatusCaptured,
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/transactions", req)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestCreateTransaction_InvalidCurrency(t *testing.T) {
	e, _ := setupTestServer()

	req := request.CreateTransactionRequest{
		TransactionID:       "txn_badcur",
		AuthorizationAmount: decimal.NewFromFloat(100),
		Currency:            models.Currency("ZZZ"),
		Processor:           "stripe",
		Timestamp:           time.Now(),
		CustomerCountry:     "US",
		PaymentMethod:       models.PaymentMethodCreditCard,
		Status:              models.TransactionStatusCaptured,
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/transactions", req)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestCreateTransaction_InvalidPaymentMethod(t *testing.T) {
	e, _ := setupTestServer()

	req := request.CreateTransactionRequest{
		TransactionID:       "txn_badpm",
		AuthorizationAmount: decimal.NewFromFloat(100),
		Currency:            models.CurrencyUSD,
		Processor:           "stripe",
		Timestamp:           time.Now(),
		CustomerCountry:     "US",
		PaymentMethod:       models.PaymentMethod("bitcoin"),
		Status:              models.TransactionStatusCaptured,
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/transactions", req)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestCreateTransaction_InvalidCountryCode(t *testing.T) {
	e, _ := setupTestServer()

	req := request.CreateTransactionRequest{
		TransactionID:       "txn_badcc",
		AuthorizationAmount: decimal.NewFromFloat(100),
		Currency:            models.CurrencyUSD,
		Processor:           "stripe",
		Timestamp:           time.Now(),
		CustomerCountry:     "USA", // 3 chars, should be 2
		PaymentMethod:       models.PaymentMethodCreditCard,
		Status:              models.TransactionStatusCaptured,
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/transactions", req)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestCreateTransaction_DuplicateReturnsConflict(t *testing.T) {
	e, _ := setupTestServer()

	req := request.CreateTransactionRequest{
		TransactionID:       "txn_dup_test",
		AuthorizationAmount: decimal.NewFromFloat(100),
		Currency:            models.CurrencyUSD,
		Processor:           "stripe",
		Timestamp:           time.Now(),
		CustomerCountry:     "US",
		PaymentMethod:       models.PaymentMethodCreditCard,
		Status:              models.TransactionStatusCaptured,
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/transactions", req)
	assert.Equal(t, http.StatusCreated, rec.Code)

	rec = doRequest(e, http.MethodPost, "/api/v1/transactions", req)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestBatchTransaction_EmptyArray(t *testing.T) {
	e, _ := setupTestServer()

	req := request.BatchTransactionRequest{
		Transactions: []request.CreateTransactionRequest{},
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/transactions/batch", req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// =====================================================
// Settlement Validation Tests
// =====================================================

func TestCreateSettlement_MissingBatchID(t *testing.T) {
	e, _ := setupTestServer()

	req := request.CreateSettlementRequest{
		BatchID:        "",
		Processor:      "stripe",
		SettlementDate: time.Now(),
		Currency:       models.CurrencyUSD,
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/settlements", req)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestCreateSettlement_InvalidCurrency(t *testing.T) {
	e, _ := setupTestServer()

	req := request.CreateSettlementRequest{
		BatchID:        "batch_bad",
		Processor:      "stripe",
		SettlementDate: time.Now(),
		Currency:       models.Currency("XXX"),
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/settlements", req)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestCreateSettlement_DuplicateBatchID(t *testing.T) {
	e, _ := setupTestServer()

	req := request.CreateSettlementRequest{
		BatchID:           "batch_dup",
		Processor:         "stripe",
		SettlementDate:    time.Now(),
		Currency:          models.CurrencyUSD,
		TotalGross:        decimal.NewFromFloat(0),
		TotalNet:          decimal.NewFromFloat(0),
		TotalFees:         decimal.NewFromFloat(0),
		ReportGeneratedAt: time.Now(),
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/settlements", req)
	assert.Equal(t, http.StatusCreated, rec.Code)

	rec = doRequest(e, http.MethodPost, "/api/v1/settlements", req)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestBatchSettlement_EmptyArray(t *testing.T) {
	e, _ := setupTestServer()

	req := request.BatchSettlementRequest{
		Settlements: []request.CreateSettlementRequest{},
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/settlements/batch", req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// =====================================================
// GET Not Found Tests
// =====================================================

func TestGetTransaction_NotFound(t *testing.T) {
	e, _ := setupTestServer()
	rec := doRequest(e, http.MethodGet, "/api/v1/transactions/nonexistent", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	var errResp response.ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &errResp)
	assert.Equal(t, "NOT_FOUND", errResp.Error.Code)
}

func TestGetSettlement_NotFound(t *testing.T) {
	e, _ := setupTestServer()
	rec := doRequest(e, http.MethodGet, "/api/v1/settlements/nonexistent", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetReport_NotFound(t *testing.T) {
	e, _ := setupTestServer()
	rec := doRequest(e, http.MethodGet, "/api/v1/reconciliation/reports/nonexistent", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// =====================================================
// Reconciliation Validation Tests
// =====================================================

func TestReconciliation_MissingPeriodStart(t *testing.T) {
	e, _ := setupTestServer()

	req := request.RunReconciliationRequest{
		PeriodEnd: time.Now(),
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/reconciliation/run", req)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestReconciliation_PeriodEndBeforeStart(t *testing.T) {
	e, _ := setupTestServer()

	req := request.RunReconciliationRequest{
		PeriodStart: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/reconciliation/run", req)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestReconciliation_EmptyData_ReturnsEmptyReport(t *testing.T) {
	e, _ := setupTestServer()

	req := request.RunReconciliationRequest{
		PeriodStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/reconciliation/run", req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var report models.ReconciliationReport
	json.Unmarshal(rec.Body.Bytes(), &report)
	assert.Equal(t, 0, report.TotalTransactions)
	assert.Empty(t, report.ProblematicTransactions)
}

// =====================================================
// Pagination Tests
// =====================================================

func TestListTransactions_DefaultPagination(t *testing.T) {
	e, _ := setupTestServer()

	rec := doRequest(e, http.MethodGet, "/api/v1/transactions", nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.PaginatedResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 50, resp.PageSize)
	assert.Equal(t, 0, resp.TotalCount)
}

func TestListTransactions_CustomPagination(t *testing.T) {
	e, _ := setupTestServer()

	rec := doRequest(e, http.MethodGet, "/api/v1/transactions?page=2&page_size=10", nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.PaginatedResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.Equal(t, 2, resp.Page)
	assert.Equal(t, 10, resp.PageSize)
}
