package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/controllers"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/repository/memory"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/request"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/response"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/services"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"time"
)

func setupTestServer() (*echo.Echo, *services.ReconciliationService) {
	e := echo.New()

	txnRepo := memory.NewTransactionRepo()
	stlRepo := memory.NewSettlementRepo()
	discRepo := memory.NewDiscrepancyRepo()
	rptRepo := memory.NewReportRepo()
	feeRepo := memory.NewFeeRuleRepo()

	txnService := services.NewTransactionService(txnRepo)
	stlService := services.NewSettlementService(stlRepo)
	reconService := services.NewReconciliationService(txnRepo, stlRepo, discRepo, rptRepo, feeRepo)

	txnCtrl := controllers.NewTransactionController(txnService)
	stlCtrl := controllers.NewSettlementController(stlService)
	reconCtrl := controllers.NewReconciliationController(reconService)

	v1 := e.Group("/api/v1")
	v1.POST("/transactions", txnCtrl.Create)
	v1.POST("/transactions/batch", txnCtrl.CreateBatch)
	v1.GET("/transactions", txnCtrl.List)
	v1.GET("/transactions/:id", txnCtrl.GetByID)
	v1.POST("/settlements", stlCtrl.Create)
	v1.POST("/settlements/batch", stlCtrl.CreateBatch)
	v1.GET("/settlements", stlCtrl.List)
	v1.GET("/settlements/:id", stlCtrl.GetByBatchID)
	v1.POST("/reconciliation/run", reconCtrl.RunReconciliation)
	v1.GET("/reconciliation/reports/:id", reconCtrl.GetReport)
	v1.GET("/reconciliation/discrepancies", reconCtrl.ListDiscrepancies)

	return e, reconService
}

func doRequest(e *echo.Echo, method, path string, body interface{}) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestFullReconciliationWorkflow(t *testing.T) {
	e, _ := setupTestServer()

	jan15 := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	jan20 := time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC)
	feb13 := time.Date(2026, 2, 13, 23, 59, 59, 0, time.UTC)

	// Step 1: Ingest transactions (includes one that will be "missing" from settlements)
	txnBatch := request.BatchTransactionRequest{
		Transactions: []request.CreateTransactionRequest{
			{
				TransactionID:       "txn_001",
				AuthorizationAmount: decimal.NewFromFloat(1000),
				Currency:            models.CurrencyUSD,
				Processor:           "stripe",
				Timestamp:           jan15,
				CustomerCountry:     "US",
				PaymentMethod:       models.PaymentMethodCreditCard,
				Status:              models.TransactionStatusCaptured,
			},
			{
				TransactionID:       "txn_002",
				AuthorizationAmount: decimal.NewFromFloat(2000),
				Currency:            models.CurrencyUSD,
				Processor:           "stripe",
				Timestamp:           jan15,
				CustomerCountry:     "MX",
				PaymentMethod:       models.PaymentMethodCreditCard,
				Status:              models.TransactionStatusCaptured,
			},
			{
				TransactionID:       "txn_003",
				AuthorizationAmount: decimal.NewFromFloat(500),
				Currency:            models.CurrencyUSD,
				Processor:           "adyen",
				Timestamp:           jan15,
				CustomerCountry:     "BR",
				PaymentMethod:       models.PaymentMethodDebitCard,
				Status:              models.TransactionStatusCaptured,
			},
		},
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/transactions/batch", txnBatch)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var batchResp response.BatchIngestionResponse
	json.Unmarshal(rec.Body.Bytes(), &batchResp)
	assert.Equal(t, 3, batchResp.Ingested)
	assert.Equal(t, 0, batchResp.DuplicatesSkipped)

	// Step 2: Ingest settlement (only txn_001 settled, txn_002 missing, txn_003 with wrong amount)
	settlement := request.CreateSettlementRequest{
		BatchID:        "batch_001",
		Processor:      "stripe",
		SettlementDate: jan20,
		Currency:       models.CurrencyUSD,
		Transactions: []request.SettlementTransactionRequest{
			{
				TransactionID:  "txn_001",
				GrossAmount:    decimal.NewFromFloat(1000),
				NetAmount:      decimal.NewFromFloat(970.70),
				Fees:           []request.FeeItemRequest{{Type: "processing", Amount: decimal.NewFromFloat(29.30)}},
				SettlementDate: jan20,
			},
			{
				TransactionID:  "txn_ORPHAN",
				GrossAmount:    decimal.NewFromFloat(5000),
				NetAmount:      decimal.NewFromFloat(4855),
				Fees:           []request.FeeItemRequest{{Type: "processing", Amount: decimal.NewFromFloat(145)}},
				SettlementDate: jan20,
			},
		},
		TotalGross:        decimal.NewFromFloat(6000),
		TotalNet:          decimal.NewFromFloat(5825.70),
		TotalFees:         decimal.NewFromFloat(174.30),
		ReserveHoldAmount: decimal.NewFromFloat(0),
		ReportGeneratedAt: jan20,
	}

	rec = doRequest(e, http.MethodPost, "/api/v1/settlements", settlement)
	assert.Equal(t, http.StatusCreated, rec.Code)

	// Step 3: Run reconciliation
	reconReq := request.RunReconciliationRequest{
		PeriodStart:             jan15.Add(-time.Hour),
		PeriodEnd:               feb13,
		IncludeTemporalAnalysis: true,
	}

	rec = doRequest(e, http.MethodPost, "/api/v1/reconciliation/run", reconReq)
	assert.Equal(t, http.StatusOK, rec.Code)

	var report models.ReconciliationReport
	err := json.Unmarshal(rec.Body.Bytes(), &report)
	require.NoError(t, err)

	// Verify report structure
	assert.Equal(t, "COMPLETED", report.Status)
	assert.Equal(t, 3, report.TotalTransactions)
	assert.NotEmpty(t, report.ReportID)

	// Should have discrepancies:
	// - txn_002 missing settlement (CAPTURED, >3 days old)
	// - txn_003 missing settlement (CAPTURED, >3 days old)
	// - txn_ORPHAN orphaned settlement
	assert.Greater(t, len(report.ProblematicTransactions), 0, "Expected at least one discrepancy")

	// Verify we can find the missing settlement
	foundMissing := false
	foundOrphaned := false
	for _, disc := range report.ProblematicTransactions {
		if disc.Type == models.DiscrepancyMissingSettlement {
			foundMissing = true
		}
		if disc.Type == models.DiscrepancyOrphanedSettlement {
			foundOrphaned = true
		}
	}
	assert.True(t, foundMissing, "Expected MISSING_SETTLEMENT discrepancy")
	assert.True(t, foundOrphaned, "Expected ORPHANED_SETTLEMENT discrepancy")

	// Verify temporal analysis was included
	assert.NotNil(t, report.AvgSettlementDays)

	// Step 4: Verify report is retrievable
	rec = doRequest(e, http.MethodGet, "/api/v1/reconciliation/reports/"+report.ReportID, nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Step 5: Query discrepancies with filter
	rec = doRequest(e, http.MethodGet, "/api/v1/reconciliation/discrepancies?type=MISSING_SETTLEMENT", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTransactionCRUDWorkflow(t *testing.T) {
	e, _ := setupTestServer()

	jan15 := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create single transaction
	txn := request.CreateTransactionRequest{
		TransactionID:       "txn_single",
		AuthorizationAmount: decimal.NewFromFloat(750),
		Currency:            models.CurrencyBRL,
		Processor:           "dlocal",
		Timestamp:           jan15,
		CustomerCountry:     "BR",
		PaymentMethod:       models.PaymentMethodPix,
		Status:              models.TransactionStatusCaptured,
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/transactions", txn)
	assert.Equal(t, http.StatusCreated, rec.Code)

	// Get by ID
	rec = doRequest(e, http.MethodGet, "/api/v1/transactions/txn_single", nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	var retrieved models.Transaction
	json.Unmarshal(rec.Body.Bytes(), &retrieved)
	assert.Equal(t, "txn_single", retrieved.TransactionID)
	assert.Equal(t, "750", retrieved.AuthorizationAmount.String())

	// List with filter
	rec = doRequest(e, http.MethodGet, "/api/v1/transactions?processor=dlocal", nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	var listResp response.PaginatedResponse
	json.Unmarshal(rec.Body.Bytes(), &listResp)
	assert.Equal(t, 1, listResp.TotalCount)

	// Duplicate should fail
	rec = doRequest(e, http.MethodPost, "/api/v1/transactions", txn)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestSettlementCRUDWorkflow(t *testing.T) {
	e, _ := setupTestServer()

	jan20 := time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC)

	stl := request.CreateSettlementRequest{
		BatchID:        "batch_test",
		Processor:      "stripe",
		SettlementDate: jan20,
		Currency:       models.CurrencyUSD,
		Transactions: []request.SettlementTransactionRequest{
			{
				TransactionID:  "txn_001",
				GrossAmount:    decimal.NewFromFloat(1000),
				NetAmount:      decimal.NewFromFloat(970.70),
				Fees:           []request.FeeItemRequest{{Type: "processing", Amount: decimal.NewFromFloat(29.30)}},
				SettlementDate: jan20,
			},
		},
		TotalGross:        decimal.NewFromFloat(1000),
		TotalNet:          decimal.NewFromFloat(970.70),
		TotalFees:         decimal.NewFromFloat(29.30),
		ReserveHoldAmount: decimal.NewFromFloat(0),
		ReportGeneratedAt: jan20,
	}

	rec := doRequest(e, http.MethodPost, "/api/v1/settlements", stl)
	assert.Equal(t, http.StatusCreated, rec.Code)

	// Get by batch ID
	rec = doRequest(e, http.MethodGet, "/api/v1/settlements/batch_test", nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Not found
	rec = doRequest(e, http.MethodGet, "/api/v1/settlements/nonexistent", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestReconciliationWithCleanData_NoFalsePositives(t *testing.T) {
	e, _ := setupTestServer()

	jan15 := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	jan20 := time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC)

	// All transactions perfectly match their settlements
	txnBatch := request.BatchTransactionRequest{
		Transactions: []request.CreateTransactionRequest{
			{
				TransactionID: "txn_clean_001", AuthorizationAmount: decimal.NewFromFloat(1000),
				Currency: models.CurrencyUSD, Processor: "stripe", Timestamp: jan15,
				CustomerCountry: "US", PaymentMethod: models.PaymentMethodCreditCard,
				Status: models.TransactionStatusCaptured,
			},
			{
				TransactionID: "txn_clean_002", AuthorizationAmount: decimal.NewFromFloat(2000),
				Currency: models.CurrencyUSD, Processor: "stripe", Timestamp: jan15,
				CustomerCountry: "US", PaymentMethod: models.PaymentMethodCreditCard,
				Status: models.TransactionStatusCaptured,
			},
		},
	}
	doRequest(e, http.MethodPost, "/api/v1/transactions/batch", txnBatch)

	// Stripe fee: amount*0.029 + 0.30
	// txn_001: 1000*0.029 + 0.30 = 29.30, net = 970.70
	// txn_002: 2000*0.029 + 0.30 = 58.30, net = 1941.70
	stl := request.CreateSettlementRequest{
		BatchID: "batch_clean", Processor: "stripe", SettlementDate: jan20,
		Currency: models.CurrencyUSD,
		Transactions: []request.SettlementTransactionRequest{
			{TransactionID: "txn_clean_001", GrossAmount: decimal.NewFromFloat(1000),
				NetAmount: decimal.NewFromFloat(970.70),
				Fees:      []request.FeeItemRequest{{Type: "processing", Amount: decimal.NewFromFloat(29.30)}},
				SettlementDate: jan20},
			{TransactionID: "txn_clean_002", GrossAmount: decimal.NewFromFloat(2000),
				NetAmount: decimal.NewFromFloat(1941.70),
				Fees:      []request.FeeItemRequest{{Type: "processing", Amount: decimal.NewFromFloat(58.30)}},
				SettlementDate: jan20},
		},
		TotalGross: decimal.NewFromFloat(3000), TotalNet: decimal.NewFromFloat(2912.40),
		TotalFees: decimal.NewFromFloat(87.60), ReportGeneratedAt: jan20,
	}
	doRequest(e, http.MethodPost, "/api/v1/settlements", stl)

	reconReq := request.RunReconciliationRequest{
		PeriodStart: jan15.Add(-time.Hour),
		PeriodEnd:   jan20.Add(24 * time.Hour),
	}
	rec := doRequest(e, http.MethodPost, "/api/v1/reconciliation/run", reconReq)
	assert.Equal(t, http.StatusOK, rec.Code)

	var report models.ReconciliationReport
	json.Unmarshal(rec.Body.Bytes(), &report)

	// Should have zero discrepancies for clean data
	assert.Empty(t, report.ProblematicTransactions, "Expected zero discrepancies for clean data")
}
