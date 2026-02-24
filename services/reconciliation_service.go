package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AugustoArguello/cacao-settlement-reconciliation/models"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/repository"
	"github.com/AugustoArguello/cacao-settlement-reconciliation/request"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Configurable thresholds for discrepancy detection
const (
	MissingSettlementDaysWarning  = 3
	MissingSettlementDaysCritical = 7
	FeeVarianceThresholdPercent   = 5.0
	FeeVarianceCriticalPercent    = 20.0
	FXDeviationThresholdPercent   = 3.0
	ReserveHoldWarningPercent     = 10.0
	ReserveHoldCriticalPercent    = 25.0
)

// Approximate market FX rates for validation (in production, use live rate API)
var marketFXRates = map[string]decimal.Decimal{
	"MXN_USD": decimal.NewFromFloat(0.058),
	"BRL_USD": decimal.NewFromFloat(0.195),
	"COP_USD": decimal.NewFromFloat(0.00024),
	"CLP_USD": decimal.NewFromFloat(0.0011),
	"EUR_USD": decimal.NewFromFloat(1.08),
	"USD_EUR": decimal.NewFromFloat(0.926),
	"USD_BRL": decimal.NewFromFloat(5.13),
	"USD_MXN": decimal.NewFromFloat(17.24),
}

// Default expected fee rates per processor (percentage + flat fee)
var defaultFeeRates = map[string]struct {
	RatePercent decimal.Decimal
	FlatFee     decimal.Decimal
}{
	"stripe": {decimal.NewFromFloat(2.9), decimal.NewFromFloat(0.30)},
	"adyen":  {decimal.NewFromFloat(2.5), decimal.NewFromFloat(0.12)},
	"dlocal": {decimal.NewFromFloat(3.9), decimal.NewFromFloat(0.00)},
}

type ReconciliationService struct {
	txnRepo  repository.TransactionRepository
	stlRepo  repository.SettlementRepository
	discRepo repository.DiscrepancyRepository
	rptRepo  repository.ReportRepository
	feeRepo  repository.FeeRuleRepository
}

func NewReconciliationService(
	txnRepo repository.TransactionRepository,
	stlRepo repository.SettlementRepository,
	discRepo repository.DiscrepancyRepository,
	rptRepo repository.ReportRepository,
	feeRepo repository.FeeRuleRepository,
) *ReconciliationService {
	return &ReconciliationService{
		txnRepo:  txnRepo,
		stlRepo:  stlRepo,
		discRepo: discRepo,
		rptRepo:  rptRepo,
		feeRepo:  feeRepo,
	}
}

func (s *ReconciliationService) RunReconciliation(ctx context.Context, req request.RunReconciliationRequest) (*models.ReconciliationReport, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// Fetch all transactions and settlements in the period
	transactions, err := s.txnRepo.GetByPeriod(ctx, req.PeriodStart, req.PeriodEnd, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transactions: %w", err)
	}

	settlements, err := s.stlRepo.GetByPeriod(ctx, req.PeriodStart, req.PeriodEnd, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch settlements: %w", err)
	}

	// Fetch fee rules for smart validation
	feeRules, err := s.feeRepo.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch fee rules: %w", err)
	}

	// Run all 8 discrepancy detectors
	var allDiscrepancies []models.Discrepancy

	missing := detectMissingSettlements(transactions, settlements, req.PeriodEnd)
	allDiscrepancies = append(allDiscrepancies, missing...)

	mismatches := detectAmountMismatches(transactions, settlements, feeRules)
	allDiscrepancies = append(allDiscrepancies, mismatches...)

	duplicates := detectDuplicateSettlements(settlements)
	allDiscrepancies = append(allDiscrepancies, duplicates...)

	orphaned := detectOrphanedSettlements(transactions, settlements)
	allDiscrepancies = append(allDiscrepancies, orphaned...)

	feeAnomalies := detectFeeAnomalies(transactions, settlements, feeRules)
	allDiscrepancies = append(allDiscrepancies, feeAnomalies...)

	fxIssues := detectCurrencyConversionIssues(transactions, settlements)
	allDiscrepancies = append(allDiscrepancies, fxIssues...)

	reserves := detectReserveHolds(settlements)
	allDiscrepancies = append(allDiscrepancies, reserves...)

	chargebacks := detectChargebackWithoutNotification(transactions, settlements)
	allDiscrepancies = append(allDiscrepancies, chargebacks...)

	// Store discrepancies
	if len(allDiscrepancies) > 0 {
		if err := s.discRepo.StoreBatch(ctx, allDiscrepancies); err != nil {
			return nil, fmt.Errorf("failed to store discrepancies: %w", err)
		}
	}

	// Build report
	report := buildReconciliationReport(req, transactions, settlements, allDiscrepancies)

	// Add temporal analysis if requested
	if req.IncludeTemporalAnalysis {
		avgDays, slowSettlements := analyzeSettlementTiming(transactions, settlements)
		report.AvgSettlementDays = &avgDays
		report.SlowSettlements = slowSettlements
	}

	// Store report
	if err := s.rptRepo.Store(ctx, report); err != nil {
		return nil, fmt.Errorf("failed to store report: %w", err)
	}

	return &report, nil
}

func (s *ReconciliationService) GetReport(ctx context.Context, id string) (*models.ReconciliationReport, error) {
	return s.rptRepo.GetByID(ctx, id)
}

func (s *ReconciliationService) ListReports(ctx context.Context) ([]models.ReconciliationReport, error) {
	return s.rptRepo.List(ctx)
}

func (s *ReconciliationService) ListDiscrepancies(ctx context.Context, filter repository.DiscrepancyFilter) ([]models.Discrepancy, int, error) {
	return s.discRepo.List(ctx, filter)
}

// --- Detector 1: Missing Settlements ---

func detectMissingSettlements(transactions []models.Transaction, settlements []models.SettlementReport, periodEnd time.Time) []models.Discrepancy {
	var discrepancies []models.Discrepancy

	// Build set of all transaction IDs that appear in settlements
	settledTxnIDs := make(map[string]bool)
	for _, report := range settlements {
		for _, stxn := range report.Transactions {
			settledTxnIDs[stxn.TransactionID] = true
		}
	}

	for _, txn := range transactions {
		// Only check transactions that should have been settled
		if txn.Status != models.TransactionStatusCaptured && txn.Status != models.TransactionStatusAuthorized {
			continue
		}

		if settledTxnIDs[txn.TransactionID] {
			continue
		}

		daysOld := int(periodEnd.Sub(txn.Timestamp).Hours() / 24)

		// Only flag transactions older than the warning threshold
		if daysOld <= MissingSettlementDaysWarning {
			continue
		}

		severity := models.SeverityWarning
		if daysOld > MissingSettlementDaysCritical {
			severity = models.SeverityCritical
		}

		txnID := txn.TransactionID
		processor := txn.Processor
		currency := txn.Currency
		expectedAmt := txn.AuthorizationAmount
		zero := decimal.Zero

		discrepancies = append(discrepancies, models.Discrepancy{
			ID:             uuid.New().String(),
			Type:           models.DiscrepancyMissingSettlement,
			Severity:       severity,
			TransactionID:  &txnID,
			ExpectedAmount: &expectedAmt,
			ActualAmount:   &zero,
			Difference:     &expectedAmt,
			Currency:       &currency,
			Processor:      &processor,
			Description: fmt.Sprintf("Transaction %s captured %d days ago but not found in any settlement report",
				txn.TransactionID, daysOld),
			InvestigationDetails: map[string]interface{}{
				"transaction_date":   txn.Timestamp.Format(time.RFC3339),
				"days_since_capture": daysOld,
				"processor":          txn.Processor,
				"payment_method":     string(txn.PaymentMethod),
				"action":             "Contact processor to inquire about settlement status",
			},
			DetectedAt: time.Now().UTC(),
		})
	}

	return discrepancies
}

// --- Detector 2: Amount Mismatches ---

func detectAmountMismatches(transactions []models.Transaction, settlements []models.SettlementReport, feeRules []models.FeeRule) []models.Discrepancy {
	var discrepancies []models.Discrepancy

	txnLookup := make(map[string]models.Transaction)
	for _, txn := range transactions {
		txnLookup[txn.TransactionID] = txn
	}

	for _, report := range settlements {
		for _, stxn := range report.Transactions {
			txn, exists := txnLookup[stxn.TransactionID]
			if !exists {
				continue // Handled by orphaned detection
			}

			// Calculate expected fees
			expectedFeeRate := getExpectedFeeRate(txn.Processor, feeRules)
			expectedFees := txn.AuthorizationAmount.Mul(expectedFeeRate.RatePercent.Div(decimal.NewFromInt(100))).Add(expectedFeeRate.FlatFee)
			expectedNet := txn.AuthorizationAmount.Sub(expectedFees)

			// Tolerance: max of $0.01 or 0.5% of transaction amount
			tolerance := decimal.Max(
				decimal.NewFromFloat(0.01),
				txn.AuthorizationAmount.Mul(decimal.NewFromFloat(0.005)),
			)

			difference := stxn.NetAmount.Sub(expectedNet)

			if difference.Abs().GreaterThan(tolerance) {
				severity := models.SeverityWarning
				if difference.Abs().GreaterThan(decimal.NewFromInt(100)) {
					severity = models.SeverityCritical
				}

				txnID := stxn.TransactionID
				batchID := report.BatchID
				processor := txn.Processor
				currency := txn.Currency

				totalActualFees := decimal.Zero
				for _, f := range stxn.Fees {
					totalActualFees = totalActualFees.Add(f.Amount)
				}

				feeBreakdown := make([]map[string]string, len(stxn.Fees))
				for i, f := range stxn.Fees {
					feeBreakdown[i] = map[string]string{
						"type":   f.Type,
						"amount": f.Amount.String(),
					}
				}

				discrepancies = append(discrepancies, models.Discrepancy{
					ID:             uuid.New().String(),
					Type:           models.DiscrepancyAmountMismatch,
					Severity:       severity,
					TransactionID:  &txnID,
					BatchID:        &batchID,
					ExpectedAmount: &expectedNet,
					ActualAmount:   &stxn.NetAmount,
					Difference:     &difference,
					Currency:       &currency,
					Processor:      &processor,
					Description: fmt.Sprintf("Net settlement %s differs from expected %s by %s",
						stxn.NetAmount.String(), expectedNet.String(), difference.String()),
					InvestigationDetails: map[string]interface{}{
						"authorized_amount": txn.AuthorizationAmount.String(),
						"gross_settled":     stxn.GrossAmount.String(),
						"net_settled":       stxn.NetAmount.String(),
						"expected_fees":     expectedFees.String(),
						"actual_fees":       totalActualFees.String(),
						"fee_breakdown":     feeBreakdown,
						"action":            "Verify fee schedule with processor; check for hidden charges",
					},
					DetectedAt: time.Now().UTC(),
				})
			}
		}
	}

	return discrepancies
}

// --- Detector 3: Duplicate Settlements ---

func detectDuplicateSettlements(settlements []models.SettlementReport) []models.Discrepancy {
	var discrepancies []models.Discrepancy

	type batchEntry struct {
		BatchID        string
		GrossAmount    decimal.Decimal
		NetAmount      decimal.Decimal
		SettlementDate time.Time
	}

	txnToBatches := make(map[string][]batchEntry)

	for _, report := range settlements {
		for _, stxn := range report.Transactions {
			txnToBatches[stxn.TransactionID] = append(txnToBatches[stxn.TransactionID], batchEntry{
				BatchID:        report.BatchID,
				GrossAmount:    stxn.GrossAmount,
				NetAmount:      stxn.NetAmount,
				SettlementDate: stxn.SettlementDate,
			})
		}
	}

	for txnID, batches := range txnToBatches {
		if len(batches) <= 1 {
			continue
		}

		totalSettled := decimal.Zero
		batchIDs := make([]string, len(batches))
		occurrences := make([]map[string]string, len(batches))

		for i, b := range batches {
			totalSettled = totalSettled.Add(b.GrossAmount)
			batchIDs[i] = b.BatchID
			occurrences[i] = map[string]string{
				"batch_id":        b.BatchID,
				"gross_amount":    b.GrossAmount.String(),
				"settlement_date": b.SettlementDate.Format("2006-01-02"),
			}
		}

		expectedAmount := batches[0].GrossAmount
		difference := totalSettled.Sub(expectedAmount)

		id := txnID
		discrepancies = append(discrepancies, models.Discrepancy{
			ID:             uuid.New().String(),
			Type:           models.DiscrepancyDuplicateSettlement,
			Severity:       models.SeverityCritical,
			TransactionID:  &id,
			ExpectedAmount: &expectedAmount,
			ActualAmount:   &totalSettled,
			Difference:     &difference,
			Description: fmt.Sprintf("Transaction %s settled in %d batches (total: %s)",
				txnID, len(batches), totalSettled.String()),
			InvestigationDetails: map[string]interface{}{
				"occurrences": occurrences,
				"batch_ids":   batchIDs,
				"action":      "Request reversal for duplicate settlement from processor",
			},
			DetectedAt: time.Now().UTC(),
		})
	}

	return discrepancies
}

// --- Detector 4: Orphaned Settlements ---

func detectOrphanedSettlements(transactions []models.Transaction, settlements []models.SettlementReport) []models.Discrepancy {
	var discrepancies []models.Discrepancy

	knownTxnIDs := make(map[string]bool)
	for _, txn := range transactions {
		knownTxnIDs[txn.TransactionID] = true
	}

	for _, report := range settlements {
		for _, stxn := range report.Transactions {
			if knownTxnIDs[stxn.TransactionID] {
				continue
			}

			txnID := stxn.TransactionID
			batchID := report.BatchID
			processor := report.Processor
			currency := report.Currency

			discrepancies = append(discrepancies, models.Discrepancy{
				ID:            uuid.New().String(),
				Type:          models.DiscrepancyOrphanedSettlement,
				Severity:      models.SeverityWarning,
				TransactionID: &txnID,
				BatchID:       &batchID,
				ActualAmount:  &stxn.NetAmount,
				Currency:      &currency,
				Processor:     &processor,
				Description: fmt.Sprintf("Settlement for %s in batch %s has no matching internal transaction",
					stxn.TransactionID, report.BatchID),
				InvestigationDetails: map[string]interface{}{
					"gross_amount":    stxn.GrossAmount.String(),
					"net_amount":      stxn.NetAmount.String(),
					"settlement_date": stxn.SettlementDate.Format("2006-01-02"),
					"processor":       report.Processor,
					"action":          "Check if transaction was ingested through a different system or if this is a processor error",
				},
				DetectedAt: time.Now().UTC(),
			})
		}
	}

	return discrepancies
}

// --- Detector 5: Fee Anomalies ---

func detectFeeAnomalies(transactions []models.Transaction, settlements []models.SettlementReport, feeRules []models.FeeRule) []models.Discrepancy {
	var discrepancies []models.Discrepancy

	txnLookup := make(map[string]models.Transaction)
	for _, txn := range transactions {
		txnLookup[txn.TransactionID] = txn
	}

	for _, report := range settlements {
		for _, stxn := range report.Transactions {
			txn, exists := txnLookup[stxn.TransactionID]
			if !exists || stxn.GrossAmount.IsZero() {
				continue
			}

			actualFeeTotal := decimal.Zero
			for _, f := range stxn.Fees {
				actualFeeTotal = actualFeeTotal.Add(f.Amount)
			}

			actualFeeRate := actualFeeTotal.Div(stxn.GrossAmount).Mul(decimal.NewFromInt(100))

			expectedRate := getExpectedFeeRate(txn.Processor, feeRules)
			expectedFeeAmount := stxn.GrossAmount.Mul(expectedRate.RatePercent.Div(decimal.NewFromInt(100))).Add(expectedRate.FlatFee)
			expectedFeeRatePct := expectedFeeAmount.Div(stxn.GrossAmount).Mul(decimal.NewFromInt(100))

			if expectedFeeRatePct.IsZero() {
				continue
			}

			variancePct := actualFeeRate.Sub(expectedFeeRatePct).Abs().Div(expectedFeeRatePct).Mul(decimal.NewFromInt(100))

			threshold := decimal.NewFromFloat(FeeVarianceThresholdPercent)
			if variancePct.LessThanOrEqual(threshold) {
				continue
			}

			severity := models.SeverityWarning
			if variancePct.GreaterThan(decimal.NewFromFloat(FeeVarianceCriticalPercent)) {
				severity = models.SeverityCritical
			}

			txnID := stxn.TransactionID
			batchID := report.BatchID
			processor := txn.Processor
			difference := actualFeeTotal.Sub(expectedFeeAmount)

			feeBreakdown := make([]map[string]string, len(stxn.Fees))
			for i, f := range stxn.Fees {
				feeBreakdown[i] = map[string]string{
					"type":   f.Type,
					"amount": f.Amount.String(),
				}
			}

			discrepancies = append(discrepancies, models.Discrepancy{
				ID:             uuid.New().String(),
				Type:           models.DiscrepancyFeeAnomaly,
				Severity:       severity,
				TransactionID:  &txnID,
				BatchID:        &batchID,
				ExpectedAmount: &expectedFeeAmount,
				ActualAmount:   &actualFeeTotal,
				Difference:     &difference,
				Processor:      &processor,
				Description: fmt.Sprintf("Fee rate %.2f%% deviates %.1f%% from expected %.2f%%",
					actualFeeRate.InexactFloat64(), variancePct.InexactFloat64(), expectedFeeRatePct.InexactFloat64()),
				InvestigationDetails: map[string]interface{}{
					"expected_rate_pct": expectedFeeRatePct.String(),
					"actual_rate_pct":   actualFeeRate.String(),
					"variance_pct":      variancePct.String(),
					"fee_breakdown":     feeBreakdown,
					"payment_method":    string(txn.PaymentMethod),
					"country":           txn.CustomerCountry,
					"action":            "Review processor fee schedule; check for rate changes or miscategorized transaction types",
				},
				DetectedAt: time.Now().UTC(),
			})
		}
	}

	return discrepancies
}

// --- Detector 6: Currency Conversion Issues ---

func detectCurrencyConversionIssues(transactions []models.Transaction, settlements []models.SettlementReport) []models.Discrepancy {
	var discrepancies []models.Discrepancy

	txnLookup := make(map[string]models.Transaction)
	for _, txn := range transactions {
		txnLookup[txn.TransactionID] = txn
	}

	for _, report := range settlements {
		for _, stxn := range report.Transactions {
			txn, exists := txnLookup[stxn.TransactionID]
			if !exists {
				continue
			}

			// Case 1: Transaction has explicit original_currency different from settlement
			if txn.OriginalCurrency != nil && *txn.OriginalCurrency != report.Currency {
				if txn.OriginalAmount != nil && txn.OriginalAmount.GreaterThan(decimal.Zero) {
					impliedRate := stxn.GrossAmount.Div(*txn.OriginalAmount)
					rateKey := fmt.Sprintf("%s_%s", *txn.OriginalCurrency, report.Currency)
					marketRate, hasRate := marketFXRates[rateKey]

					if hasRate && marketRate.GreaterThan(decimal.Zero) {
						deviation := impliedRate.Sub(marketRate).Abs().Div(marketRate).Mul(decimal.NewFromInt(100))

						if deviation.GreaterThan(decimal.NewFromFloat(FXDeviationThresholdPercent)) {
							severity := models.SeverityWarning
							if deviation.GreaterThan(decimal.NewFromFloat(10)) {
								severity = models.SeverityCritical
							}

							txnID := stxn.TransactionID
							difference := stxn.GrossAmount.Sub(txn.OriginalAmount.Mul(marketRate))

							discrepancies = append(discrepancies, models.Discrepancy{
								ID:            uuid.New().String(),
								Type:          models.DiscrepancyCurrencyConversion,
								Severity:      severity,
								TransactionID: &txnID,
								Difference:    &difference,
								Description: fmt.Sprintf("FX rate %s deviates %.1f%% from market rate %s",
									impliedRate.StringFixed(6), deviation.InexactFloat64(), marketRate.StringFixed(6)),
								InvestigationDetails: map[string]interface{}{
									"original_currency":  string(*txn.OriginalCurrency),
									"settlement_currency": string(report.Currency),
									"original_amount":    txn.OriginalAmount.String(),
									"settled_gross":      stxn.GrossAmount.String(),
									"implied_rate":       impliedRate.StringFixed(6),
									"market_rate":        marketRate.StringFixed(6),
									"deviation_pct":      deviation.String(),
									"action":             "Verify FX rate with processor; check for additional FX margin fees",
								},
								DetectedAt: time.Now().UTC(),
							})
						}
					}
				}
			} else if txn.Currency == report.Currency {
				// Case 2: Same currency but gross != authorized
				if !stxn.GrossAmount.Equal(txn.AuthorizationAmount) {
					diff := stxn.GrossAmount.Sub(txn.AuthorizationAmount)
					if diff.Abs().GreaterThan(decimal.NewFromFloat(0.01)) {
						txnID := stxn.TransactionID
						expectedAmt := txn.AuthorizationAmount
						actualAmt := stxn.GrossAmount

						discrepancies = append(discrepancies, models.Discrepancy{
							ID:             uuid.New().String(),
							Type:           models.DiscrepancyCurrencyConversion,
							Severity:       models.SeverityInfo,
							TransactionID:  &txnID,
							ExpectedAmount: &expectedAmt,
							ActualAmount:   &actualAmt,
							Difference:     &diff,
							Description: fmt.Sprintf("Gross settled amount %s differs from authorized %s with same currency (possible hidden FX conversion)",
								stxn.GrossAmount.String(), txn.AuthorizationAmount.String()),
							InvestigationDetails: map[string]interface{}{
								"authorized":    txn.AuthorizationAmount.String(),
								"gross_settled": stxn.GrossAmount.String(),
								"currency":      string(txn.Currency),
								"action":        "Investigate if an undisclosed currency conversion occurred",
							},
							DetectedAt: time.Now().UTC(),
						})
					}
				}
			}
		}
	}

	return discrepancies
}

// --- Detector 7: Reserve Holds ---

func detectReserveHolds(settlements []models.SettlementReport) []models.Discrepancy {
	var discrepancies []models.Discrepancy

	for _, report := range settlements {
		if report.ReserveHoldAmount.IsZero() || report.ReserveHoldAmount.IsNegative() {
			continue
		}

		if report.TotalGross.IsZero() {
			continue
		}

		holdPct := report.ReserveHoldAmount.Div(report.TotalGross).Mul(decimal.NewFromInt(100))

		severity := models.SeverityInfo
		if holdPct.GreaterThan(decimal.NewFromFloat(ReserveHoldWarningPercent)) {
			severity = models.SeverityWarning
		}
		if holdPct.GreaterThan(decimal.NewFromFloat(ReserveHoldCriticalPercent)) {
			severity = models.SeverityCritical
		}

		batchID := report.BatchID
		processor := report.Processor
		currency := report.Currency
		expectedNet := report.TotalNet
		actualNet := report.TotalNet.Sub(report.ReserveHoldAmount)

		discrepancies = append(discrepancies, models.Discrepancy{
			ID:             uuid.New().String(),
			Type:           models.DiscrepancyReserveHold,
			Severity:       severity,
			BatchID:        &batchID,
			ExpectedAmount: &expectedNet,
			ActualAmount:   &actualNet,
			Difference:     &report.ReserveHoldAmount,
			Currency:       &currency,
			Processor:      &processor,
			Description: fmt.Sprintf("Reserve hold of %s (%.1f%% of gross) in batch %s",
				report.ReserveHoldAmount.String(), holdPct.InexactFloat64(), report.BatchID),
			InvestigationDetails: map[string]interface{}{
				"batch_id":         report.BatchID,
				"total_gross":      report.TotalGross.String(),
				"total_net":        report.TotalNet.String(),
				"reserve_amount":   report.ReserveHoldAmount.String(),
				"reserve_pct":      holdPct.String(),
				"settlement_date":  report.SettlementDate.Format("2006-01-02"),
				"action":           "Review reserve schedule with processor; verify release timeline",
			},
			DetectedAt: time.Now().UTC(),
		})
	}

	return discrepancies
}

// --- Detector 8: Chargeback Without Notification ---

func detectChargebackWithoutNotification(transactions []models.Transaction, settlements []models.SettlementReport) []models.Discrepancy {
	var discrepancies []models.Discrepancy

	txnLookup := make(map[string]models.Transaction)
	for _, txn := range transactions {
		txnLookup[txn.TransactionID] = txn
	}

	for _, report := range settlements {
		for _, stxn := range report.Transactions {
			txn, exists := txnLookup[stxn.TransactionID]
			if !exists {
				continue
			}

			// Check for chargeback-related fees
			var chargebackFees []models.FeeItem
			for _, f := range stxn.Fees {
				feeType := strings.ToLower(f.Type)
				if strings.Contains(feeType, "chargeback") ||
					strings.Contains(feeType, "dispute") ||
					strings.Contains(feeType, "reversal") ||
					strings.Contains(feeType, "representment") {
					chargebackFees = append(chargebackFees, f)
				}
			}

			if len(chargebackFees) > 0 && txn.Status != models.TransactionStatusChargedback {
				totalCBFees := decimal.Zero
				for _, f := range chargebackFees {
					totalCBFees = totalCBFees.Add(f.Amount)
				}

				txnID := stxn.TransactionID
				batchID := report.BatchID
				processor := report.Processor
				zero := decimal.Zero

				cbFeeDetails := make([]map[string]string, len(chargebackFees))
				for i, f := range chargebackFees {
					cbFeeDetails[i] = map[string]string{
						"type":   f.Type,
						"amount": f.Amount.String(),
					}
				}

				discrepancies = append(discrepancies, models.Discrepancy{
					ID:             uuid.New().String(),
					Type:           models.DiscrepancyChargebackWithoutNotification,
					Severity:       models.SeverityCritical,
					TransactionID:  &txnID,
					BatchID:        &batchID,
					ExpectedAmount: &zero,
					ActualAmount:   &totalCBFees,
					Difference:     &totalCBFees,
					Processor:      &processor,
					Description: fmt.Sprintf("Chargeback deduction of %s found in settlement but transaction status is %s (not CHARGEDBACK)",
						totalCBFees.String(), string(txn.Status)),
					InvestigationDetails: map[string]interface{}{
						"internal_status":  string(txn.Status),
						"chargeback_fees":  cbFeeDetails,
						"settlement_net":   stxn.NetAmount.String(),
						"action":           "Check webhook/notification logs; update internal status; investigate notification delivery failure",
					},
					DetectedAt: time.Now().UTC(),
				})
			}

			// Also check for negative net (full chargeback reversal)
			if stxn.NetAmount.IsNegative() && txn.Status != models.TransactionStatusChargedback {
				txnID := stxn.TransactionID
				batchID := report.BatchID
				processor := report.Processor
				zero := decimal.Zero
				absNet := stxn.NetAmount.Abs()

				discrepancies = append(discrepancies, models.Discrepancy{
					ID:             uuid.New().String(),
					Type:           models.DiscrepancyChargebackWithoutNotification,
					Severity:       models.SeverityCritical,
					TransactionID:  &txnID,
					BatchID:        &batchID,
					ExpectedAmount: &zero,
					ActualAmount:   &stxn.NetAmount,
					Difference:     &absNet,
					Processor:      &processor,
					Description: fmt.Sprintf("Negative settlement of %s indicates full chargeback reversal without internal notification",
						stxn.NetAmount.String()),
					InvestigationDetails: map[string]interface{}{
						"net_amount":      stxn.NetAmount.String(),
						"gross_amount":    stxn.GrossAmount.String(),
						"internal_status": string(txn.Status),
						"action":          "URGENT: Full reversal detected. Update transaction status and investigate notification pipeline failure.",
					},
					DetectedAt: time.Now().UTC(),
				})
			}
		}
	}

	return discrepancies
}

// --- Temporal Analysis (Stretch) ---

func analyzeSettlementTiming(transactions []models.Transaction, settlements []models.SettlementReport) (decimal.Decimal, []models.SlowSettlement) {
	txnLookup := make(map[string]models.Transaction)
	for _, txn := range transactions {
		txnLookup[txn.TransactionID] = txn
	}

	var durations []int
	var slowSettlements []models.SlowSettlement

	for _, report := range settlements {
		for _, stxn := range report.Transactions {
			txn, exists := txnLookup[stxn.TransactionID]
			if !exists {
				continue
			}

			days := int(stxn.SettlementDate.Sub(txn.Timestamp).Hours() / 24)
			if days < 0 {
				days = 0
			}
			durations = append(durations, days)

			if days > 10 {
				slowSettlements = append(slowSettlements, models.SlowSettlement{
					TransactionID:   stxn.TransactionID,
					DaysToSettle:    days,
					Processor:       txn.Processor,
					PaymentMethod:   string(txn.PaymentMethod),
					TransactionDate: txn.Timestamp.Format(time.RFC3339),
					SettlementDate:  stxn.SettlementDate.Format("2006-01-02"),
					Amount:          txn.AuthorizationAmount.String(),
				})
			}
		}
	}

	avgDays := decimal.Zero
	if len(durations) > 0 {
		total := 0
		for _, d := range durations {
			total += d
		}
		avgDays = decimal.NewFromFloat(float64(total) / float64(len(durations))).Round(1)
	}

	return avgDays, slowSettlements
}

// --- Report Builder ---

func buildReconciliationReport(req request.RunReconciliationRequest, transactions []models.Transaction, settlements []models.SettlementReport, discrepancies []models.Discrepancy) models.ReconciliationReport {
	totalExpected := decimal.Zero
	for _, txn := range transactions {
		if txn.Status == models.TransactionStatusCaptured || txn.Status == models.TransactionStatusAuthorized {
			totalExpected = totalExpected.Add(txn.AuthorizationAmount)
		}
	}

	totalActual := decimal.Zero
	totalSettlementTxns := 0
	for _, report := range settlements {
		totalActual = totalActual.Add(report.TotalNet)
		totalSettlementTxns += len(report.Transactions)
	}

	netDiscrepancy := totalActual.Sub(totalExpected)
	discPct := decimal.Zero
	if !totalExpected.IsZero() {
		discPct = netDiscrepancy.Abs().Div(totalExpected).Mul(decimal.NewFromInt(100)).Round(2)
	}

	// Build discrepancy breakdown
	typeMap := make(map[models.DiscrepancyType]*models.DiscrepancySummary)
	for _, disc := range discrepancies {
		summary, exists := typeMap[disc.Type]
		if !exists {
			summary = &models.DiscrepancySummary{
				Type:              disc.Type,
				Count:             0,
				TotalAmount:       decimal.Zero,
				SeverityBreakdown: make(map[models.Severity]int),
			}
			typeMap[disc.Type] = summary
		}
		summary.Count++
		if disc.Difference != nil {
			summary.TotalAmount = summary.TotalAmount.Add(disc.Difference.Abs())
		}
		summary.SeverityBreakdown[disc.Severity]++
	}

	var breakdown []models.DiscrepancySummary
	for _, dt := range models.AllDiscrepancyTypes() {
		if summary, exists := typeMap[dt]; exists {
			breakdown = append(breakdown, *summary)
		}
	}

	// Build processor summary
	procMap := make(map[string]*models.ProcessorSummary)
	for _, txn := range transactions {
		if txn.Status != models.TransactionStatusCaptured && txn.Status != models.TransactionStatusAuthorized {
			continue
		}
		summary, exists := procMap[txn.Processor]
		if !exists {
			summary = &models.ProcessorSummary{
				Processor:    txn.Processor,
				ExpectedTotal: decimal.Zero,
				ActualTotal:   decimal.Zero,
			}
			procMap[txn.Processor] = summary
		}
		summary.TransactionCount++
		summary.ExpectedTotal = summary.ExpectedTotal.Add(txn.AuthorizationAmount)
	}

	for _, report := range settlements {
		if summary, exists := procMap[report.Processor]; exists {
			summary.ActualTotal = summary.ActualTotal.Add(report.TotalNet)
		}
	}

	for _, disc := range discrepancies {
		if disc.Processor != nil {
			if summary, exists := procMap[*disc.Processor]; exists {
				summary.DiscrepancyCount++
			}
		}
	}

	var processorSummary []models.ProcessorSummary
	for _, summary := range procMap {
		summary.NetDiscrepancy = summary.ActualTotal.Sub(summary.ExpectedTotal)
		processorSummary = append(processorSummary, *summary)
	}

	return models.ReconciliationReport{
		ReportID:                uuid.New().String(),
		PeriodStart:             req.PeriodStart,
		PeriodEnd:               req.PeriodEnd,
		GeneratedAt:             time.Now().UTC(),
		TotalTransactions:       len(transactions),
		TotalSettlements:        totalSettlementTxns,
		TotalExpected:           totalExpected,
		TotalActual:             totalActual,
		NetDiscrepancy:          netDiscrepancy,
		DiscrepancyPercentage:   discPct,
		DiscrepancyBreakdown:    breakdown,
		ProcessorSummary:        processorSummary,
		ProblematicTransactions: discrepancies,
		Status:                  "COMPLETED",
	}
}

// --- Helpers ---

type feeRate struct {
	RatePercent decimal.Decimal
	FlatFee     decimal.Decimal
}

func getExpectedFeeRate(processor string, rules []models.FeeRule) feeRate {
	// Check custom rules first
	for _, rule := range rules {
		if strings.EqualFold(rule.Processor, processor) {
			return feeRate{
				RatePercent: rule.RatePercent,
				FlatFee:     rule.FlatFee,
			}
		}
	}

	// Fall back to defaults
	if rate, exists := defaultFeeRates[strings.ToLower(processor)]; exists {
		return feeRate{
			RatePercent: rate.RatePercent,
			FlatFee:     rate.FlatFee,
		}
	}

	// Default generic rate
	return feeRate{
		RatePercent: decimal.NewFromFloat(3.0),
		FlatFee:     decimal.NewFromFloat(0.25),
	}
}
