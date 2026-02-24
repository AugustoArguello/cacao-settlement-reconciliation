package services

import (
	"testing"
	"time"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helper factories ---

func newTransaction(id string, amount float64, processor string, currency models.Currency, method models.PaymentMethod, status models.TransactionStatus, timestamp time.Time) models.Transaction {
	return models.Transaction{
		TransactionID:       id,
		AuthorizationAmount: decimal.NewFromFloat(amount),
		Currency:            currency,
		Processor:           processor,
		Timestamp:           timestamp,
		CustomerCountry:     "US",
		PaymentMethod:       method,
		Status:              status,
	}
}

func newSettlementReport(batchID string, processor string, currency models.Currency, settlementDate time.Time, txns []models.SettlementTransaction, reserveHold float64) models.SettlementReport {
	totalGross := decimal.Zero
	totalNet := decimal.Zero
	totalFees := decimal.Zero
	for _, t := range txns {
		totalGross = totalGross.Add(t.GrossAmount)
		totalNet = totalNet.Add(t.NetAmount)
		for _, f := range t.Fees {
			totalFees = totalFees.Add(f.Amount)
		}
	}
	return models.SettlementReport{
		BatchID:           batchID,
		Processor:         processor,
		SettlementDate:    settlementDate,
		Currency:          currency,
		Transactions:      txns,
		TotalGross:        totalGross,
		TotalNet:          totalNet,
		TotalFees:         totalFees,
		ReserveHoldAmount: decimal.NewFromFloat(reserveHold),
		ReportGeneratedAt: settlementDate,
	}
}

func newSettlementTxn(txnID string, gross, net float64, fees []models.FeeItem, settlementDate time.Time) models.SettlementTransaction {
	return models.SettlementTransaction{
		TransactionID:  txnID,
		GrossAmount:    decimal.NewFromFloat(gross),
		NetAmount:      decimal.NewFromFloat(net),
		Fees:           fees,
		SettlementDate: settlementDate,
	}
}

func fee(feeType string, amount float64) models.FeeItem {
	return models.FeeItem{
		Type:   feeType,
		Amount: decimal.NewFromFloat(amount),
	}
}

var (
	jan15 = time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	jan18 = time.Date(2026, 1, 18, 10, 0, 0, 0, time.UTC)
	jan20 = time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC)
	jan22 = time.Date(2026, 1, 22, 10, 0, 0, 0, time.UTC)
	jan25 = time.Date(2026, 1, 25, 10, 0, 0, 0, time.UTC)
	jan30 = time.Date(2026, 1, 30, 10, 0, 0, 0, time.UTC)
	feb10 = time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)
	feb13 = time.Date(2026, 2, 13, 23, 59, 59, 0, time.UTC)
)

// =====================================================
// Detector 1: Missing Settlements
// =====================================================

func TestDetectMissingSettlements_FlagsOldUnsettledTransactions(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_001", 3200, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan15),
		newTransaction("txn_002", 1500, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan18),
	}

	// Only txn_002 is in settlements
	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan22, []models.SettlementTransaction{
			newSettlementTxn("txn_002", 1500, 1456.20, []models.FeeItem{fee("processing", 43.80)}, jan22),
		}, 0),
	}

	discrepancies := detectMissingSettlements(transactions, settlements, feb13)

	require.Len(t, discrepancies, 1)
	assert.Equal(t, models.DiscrepancyMissingSettlement, discrepancies[0].Type)
	assert.Equal(t, "txn_001", *discrepancies[0].TransactionID)
	assert.Equal(t, models.SeverityCritical, discrepancies[0].Severity) // >7 days old
	assert.Equal(t, "3200", discrepancies[0].ExpectedAmount.String())
}

func TestDetectMissingSettlements_IgnoresRecentTransactions(t *testing.T) {
	// Transaction from yesterday - too recent to flag
	yesterday := feb13.Add(-24 * time.Hour)
	transactions := []models.Transaction{
		newTransaction("txn_recent", 500, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, yesterday),
	}

	discrepancies := detectMissingSettlements(transactions, nil, feb13)
	assert.Empty(t, discrepancies)
}

func TestDetectMissingSettlements_IgnoresDeclinedTransactions(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_declined", 1000, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusDeclined, jan15),
	}

	discrepancies := detectMissingSettlements(transactions, nil, feb13)
	assert.Empty(t, discrepancies)
}

func TestDetectMissingSettlements_WarningAt3to7Days(t *testing.T) {
	// Transaction is 5 days old (between 3 and 7)
	fiveDaysAgo := feb13.Add(-5 * 24 * time.Hour)
	transactions := []models.Transaction{
		newTransaction("txn_warning", 800, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, fiveDaysAgo),
	}

	discrepancies := detectMissingSettlements(transactions, nil, feb13)
	require.Len(t, discrepancies, 1)
	assert.Equal(t, models.SeverityWarning, discrepancies[0].Severity)
}

func TestDetectMissingSettlements_NoFalsePositives(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_settled", 1000, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan15),
	}
	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_settled", 1000, 970.70, []models.FeeItem{fee("processing", 29.30)}, jan20),
		}, 0),
	}

	discrepancies := detectMissingSettlements(transactions, settlements, feb13)
	assert.Empty(t, discrepancies)
}

// =====================================================
// Detector 2: Amount Mismatches
// =====================================================

func TestDetectAmountMismatches_FlagsSignificantDifference(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_001", 4500, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan15),
	}

	// Expected net for stripe: 4500 - (4500*0.029 + 0.30) = 4500 - 130.80 = 4369.20
	// Actual net: 3900 (way off)
	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_001", 4500, 3900, []models.FeeItem{fee("processing", 600)}, jan20),
		}, 0),
	}

	discrepancies := detectAmountMismatches(transactions, settlements, nil)

	require.Len(t, discrepancies, 1)
	assert.Equal(t, models.DiscrepancyAmountMismatch, discrepancies[0].Type)
	assert.Equal(t, "txn_001", *discrepancies[0].TransactionID)
	assert.Equal(t, models.SeverityCritical, discrepancies[0].Severity) // >$100 difference
}

func TestDetectAmountMismatches_ToleratesSmallDifference(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_001", 100, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan15),
	}

	// Expected net: 100 - (100*0.029 + 0.30) = 100 - 3.20 = 96.80
	// Actual net: 96.79 (within tolerance)
	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_001", 100, 96.79, []models.FeeItem{fee("processing", 3.21)}, jan20),
		}, 0),
	}

	discrepancies := detectAmountMismatches(transactions, settlements, nil)
	assert.Empty(t, discrepancies)
}

func TestDetectAmountMismatches_NoFalsePositivesOnCleanData(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_001", 1000, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan15),
	}

	// Exact expected net: 1000 - (1000*0.029 + 0.30) = 1000 - 29.30 = 970.70
	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_001", 1000, 970.70, []models.FeeItem{fee("processing", 29.30)}, jan20),
		}, 0),
	}

	discrepancies := detectAmountMismatches(transactions, settlements, nil)
	assert.Empty(t, discrepancies)
}

// =====================================================
// Detector 3: Duplicate Settlements
// =====================================================

func TestDetectDuplicateSettlements_FlagsSameTransactionInTwoBatches(t *testing.T) {
	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_dup", 3400, 3301.40, []models.FeeItem{fee("processing", 98.60)}, jan20),
		}, 0),
		newSettlementReport("batch_002", "stripe", models.CurrencyUSD, jan22, []models.SettlementTransaction{
			newSettlementTxn("txn_dup", 3400, 3301.40, []models.FeeItem{fee("processing", 98.60)}, jan22),
		}, 0),
	}

	discrepancies := detectDuplicateSettlements(settlements)

	require.Len(t, discrepancies, 1)
	assert.Equal(t, models.DiscrepancyDuplicateSettlement, discrepancies[0].Type)
	assert.Equal(t, models.SeverityCritical, discrepancies[0].Severity)
	assert.Equal(t, "txn_dup", *discrepancies[0].TransactionID)
	// Total settled should be 6800 (2x3400)
	assert.Equal(t, "6800", discrepancies[0].ActualAmount.String())
}

func TestDetectDuplicateSettlements_NoDuplicatesNoProblem(t *testing.T) {
	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_001", 1000, 970.70, nil, jan20),
			newSettlementTxn("txn_002", 2000, 1941.40, nil, jan20),
		}, 0),
	}

	discrepancies := detectDuplicateSettlements(settlements)
	assert.Empty(t, discrepancies)
}

func TestDetectDuplicateSettlements_DifferentAmountsSameID(t *testing.T) {
	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_dup", 1000, 970, nil, jan20),
		}, 0),
		newSettlementReport("batch_002", "stripe", models.CurrencyUSD, jan22, []models.SettlementTransaction{
			newSettlementTxn("txn_dup", 1500, 1455, nil, jan22),
		}, 0),
	}

	discrepancies := detectDuplicateSettlements(settlements)
	require.Len(t, discrepancies, 1)
	// Total is 2500 (1000 + 1500), expected was 1000 (first occurrence)
	assert.Equal(t, "2500", discrepancies[0].ActualAmount.String())
}

// =====================================================
// Detector 4: Orphaned Settlements
// =====================================================

func TestDetectOrphanedSettlements_FlagsUnknownTransactionIDs(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_001", 1000, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan15),
	}

	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_001", 1000, 970.70, nil, jan20),
			newSettlementTxn("txn_GHOST", 2800, 2718.80, nil, jan20), // Not in transactions
		}, 0),
	}

	discrepancies := detectOrphanedSettlements(transactions, settlements)

	require.Len(t, discrepancies, 1)
	assert.Equal(t, models.DiscrepancyOrphanedSettlement, discrepancies[0].Type)
	assert.Equal(t, "txn_GHOST", *discrepancies[0].TransactionID)
	assert.Equal(t, models.SeverityWarning, discrepancies[0].Severity)
}

func TestDetectOrphanedSettlements_NoOrphansWhenAllMatch(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_001", 1000, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan15),
	}

	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_001", 1000, 970.70, nil, jan20),
		}, 0),
	}

	discrepancies := detectOrphanedSettlements(transactions, settlements)
	assert.Empty(t, discrepancies)
}

// =====================================================
// Detector 5: Fee Anomalies
// =====================================================

func TestDetectFeeAnomalies_FlagsHighFeeVariance(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_001", 3000, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan15),
	}

	// Expected stripe fee: 3000*0.029 + 0.30 = $87.30
	// Actual fee: $187.50 (way over 5% variance)
	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_001", 3000, 2812.50, []models.FeeItem{fee("processing", 187.50)}, jan20),
		}, 0),
	}

	discrepancies := detectFeeAnomalies(transactions, settlements, nil)

	require.Len(t, discrepancies, 1)
	assert.Equal(t, models.DiscrepancyFeeAnomaly, discrepancies[0].Type)
	assert.Equal(t, "txn_001", *discrepancies[0].TransactionID)
}

func TestDetectFeeAnomalies_AcceptsNormalFees(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_001", 1000, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan15),
	}

	// Expected stripe fee: 1000*0.029 + 0.30 = $29.30
	// Actual fee: $29.30 (exact match)
	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_001", 1000, 970.70, []models.FeeItem{fee("processing", 29.30)}, jan20),
		}, 0),
	}

	discrepancies := detectFeeAnomalies(transactions, settlements, nil)
	assert.Empty(t, discrepancies)
}

func TestDetectFeeAnomalies_FlagsSuspiciouslyLowFee(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_001", 2000, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan15),
	}

	// Expected stripe fee: 2000*0.029 + 0.30 = $58.30
	// Actual fee: $5.00 (suspiciously low, >5% variance)
	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_001", 2000, 1995, []models.FeeItem{fee("processing", 5.00)}, jan20),
		}, 0),
	}

	discrepancies := detectFeeAnomalies(transactions, settlements, nil)
	require.Len(t, discrepancies, 1)
	assert.Equal(t, models.DiscrepancyFeeAnomaly, discrepancies[0].Type)
}

// =====================================================
// Detector 6: Currency Conversion Issues
// =====================================================

func TestDetectCurrencyConversion_FlagsBadFXRate(t *testing.T) {
	brl := models.CurrencyBRL
	originalAmt := decimal.NewFromFloat(10000) // 10,000 BRL
	transactions := []models.Transaction{
		{
			TransactionID:       "txn_fx",
			AuthorizationAmount: decimal.NewFromFloat(1950), // ~$1950 USD at market rate
			Currency:            models.CurrencyUSD,
			Processor:           "dlocal",
			Timestamp:           jan15,
			CustomerCountry:     "BR",
			PaymentMethod:       models.PaymentMethodPix,
			Status:              models.TransactionStatusCaptured,
			OriginalCurrency:    &brl,
			OriginalAmount:      &originalAmt,
		},
	}

	// Settled at an inflated rate (0.25 instead of market 0.195)
	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "dlocal", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_fx", 2500, 2402.50, []models.FeeItem{fee("processing", 97.50)}, jan20),
		}, 0),
	}

	discrepancies := detectCurrencyConversionIssues(transactions, settlements)
	require.Len(t, discrepancies, 1)
	assert.Equal(t, models.DiscrepancyCurrencyConversion, discrepancies[0].Type)
}

func TestDetectCurrencyConversion_SameCurrencyGrossMismatch(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_001", 1000, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan15),
	}

	// Gross amount != authorized amount in same currency
	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_001", 980, 950.70, []models.FeeItem{fee("processing", 29.30)}, jan20),
		}, 0),
	}

	discrepancies := detectCurrencyConversionIssues(transactions, settlements)
	require.Len(t, discrepancies, 1)
	assert.Equal(t, models.SeverityInfo, discrepancies[0].Severity)
}

func TestDetectCurrencyConversion_NoIssueWhenGrossMatchesAuthorized(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_001", 1000, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan15),
	}

	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_001", 1000, 970.70, []models.FeeItem{fee("processing", 29.30)}, jan20),
		}, 0),
	}

	discrepancies := detectCurrencyConversionIssues(transactions, settlements)
	assert.Empty(t, discrepancies)
}

// =====================================================
// Detector 7: Reserve Holds
// =====================================================

func TestDetectReserveHolds_FlagsReserveHoldAmount(t *testing.T) {
	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "dlocal", models.CurrencyBRL, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_001", 4200, 4065.60, []models.FeeItem{fee("processing", 134.40)}, jan20),
		}, 420), // 10% reserve hold
	}

	discrepancies := detectReserveHolds(settlements)

	require.Len(t, discrepancies, 1)
	assert.Equal(t, models.DiscrepancyReserveHold, discrepancies[0].Type)
	assert.Equal(t, "420", discrepancies[0].Difference.String())
}

func TestDetectReserveHolds_IgnoresZeroReserve(t *testing.T) {
	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_001", 1000, 970.70, nil, jan20),
		}, 0),
	}

	discrepancies := detectReserveHolds(settlements)
	assert.Empty(t, discrepancies)
}

func TestDetectReserveHolds_SeverityScalesWithPercentage(t *testing.T) {
	tests := []struct {
		name             string
		grossAmount      float64
		reserveAmount    float64
		expectedSeverity models.Severity
	}{
		{"5% reserve (info)", 10000, 500, models.SeverityInfo},
		{"15% reserve (warning)", 10000, 1500, models.SeverityWarning},
		{"30% reserve (critical)", 10000, 3000, models.SeverityCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settlements := []models.SettlementReport{
				newSettlementReport("batch_001", "dlocal", models.CurrencyUSD, jan20, []models.SettlementTransaction{
					newSettlementTxn("txn_001", tt.grossAmount, tt.grossAmount-100, nil, jan20),
				}, tt.reserveAmount),
			}

			discrepancies := detectReserveHolds(settlements)
			require.Len(t, discrepancies, 1)
			assert.Equal(t, tt.expectedSeverity, discrepancies[0].Severity)
		})
	}
}

// =====================================================
// Detector 8: Chargeback Without Notification
// =====================================================

func TestDetectChargebackWithoutNotification_FlagsChargebackFeeOnCapturedTxn(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_001", 1200, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan15),
	}

	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_001", 1200, -25, []models.FeeItem{
				fee("processing", 25),
				fee("chargeback_fee", 25),
			}, jan20),
		}, 0),
	}

	discrepancies := detectChargebackWithoutNotification(transactions, settlements)

	require.GreaterOrEqual(t, len(discrepancies), 1)
	assert.Equal(t, models.DiscrepancyChargebackWithoutNotification, discrepancies[0].Type)
	assert.Equal(t, models.SeverityCritical, discrepancies[0].Severity)
}

func TestDetectChargebackWithoutNotification_IgnoresProperlyMarkedChargeback(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_001", 1200, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusChargedback, jan15),
	}

	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_001", 1200, -25, []models.FeeItem{
				fee("chargeback_fee", 25),
			}, jan20),
		}, 0),
	}

	discrepancies := detectChargebackWithoutNotification(transactions, settlements)
	assert.Empty(t, discrepancies)
}

func TestDetectChargebackWithoutNotification_FlagsNegativeNetAmount(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_001", 500, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan15),
	}

	// Negative net = full reversal
	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan20, []models.SettlementTransaction{
			newSettlementTxn("txn_001", 500, -500, []models.FeeItem{fee("processing", 1000)}, jan20),
		}, 0),
	}

	discrepancies := detectChargebackWithoutNotification(transactions, settlements)
	require.GreaterOrEqual(t, len(discrepancies), 1)

	found := false
	for _, d := range discrepancies {
		if d.Type == models.DiscrepancyChargebackWithoutNotification {
			found = true
		}
	}
	assert.True(t, found, "Expected CHARGEBACK_WITHOUT_NOTIFICATION discrepancy")
}

// =====================================================
// Temporal Analysis (Stretch)
// =====================================================

func TestAnalyzeSettlementTiming_CalculatesAverageAndFlagsSlow(t *testing.T) {
	transactions := []models.Transaction{
		newTransaction("txn_fast", 1000, "stripe", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan15),
		newTransaction("txn_slow", 2000, "adyen", models.CurrencyUSD, models.PaymentMethodCreditCard, models.TransactionStatusCaptured, jan15),
	}

	settlements := []models.SettlementReport{
		newSettlementReport("batch_001", "stripe", models.CurrencyUSD, jan18, []models.SettlementTransaction{
			newSettlementTxn("txn_fast", 1000, 970.70, nil, jan18), // 3 days
		}, 0),
		newSettlementReport("batch_002", "adyen", models.CurrencyUSD, jan30, []models.SettlementTransaction{
			newSettlementTxn("txn_slow", 2000, 1949.88, nil, jan30), // 15 days
		}, 0),
	}

	avgDays, slowSettlements := analyzeSettlementTiming(transactions, settlements)

	assert.Equal(t, "9", avgDays.String()) // (3+15)/2 = 9
	require.Len(t, slowSettlements, 1)
	assert.Equal(t, "txn_slow", slowSettlements[0].TransactionID)
	assert.Equal(t, 15, slowSettlements[0].DaysToSettle)
}

// =====================================================
// Helper: getExpectedFeeRate
// =====================================================

func TestGetExpectedFeeRate_ReturnsProcessorDefaults(t *testing.T) {
	tests := []struct {
		processor       string
		expectedRate    string
		expectedFlatFee string
	}{
		{"stripe", "2.9", "0.3"},
		{"adyen", "2.5", "0.12"},
		{"dlocal", "3.9", "0"},
		{"unknown", "3", "0.25"}, // fallback
	}

	for _, tt := range tests {
		t.Run(tt.processor, func(t *testing.T) {
			rate := getExpectedFeeRate(tt.processor, nil)
			assert.Equal(t, tt.expectedRate, rate.RatePercent.String())
			assert.Equal(t, tt.expectedFlatFee, rate.FlatFee.String())
		})
	}
}

func TestGetExpectedFeeRate_CustomRulesOverrideDefaults(t *testing.T) {
	customRules := []models.FeeRule{
		{
			ID:          "rule_1",
			Processor:   "stripe",
			RatePercent: decimal.NewFromFloat(1.5),
			FlatFee:     decimal.NewFromFloat(0.10),
		},
	}

	rate := getExpectedFeeRate("stripe", customRules)
	assert.Equal(t, "1.5", rate.RatePercent.String())
	assert.Equal(t, "0.1", rate.FlatFee.String())
}
