// generate_test_data.go generates deterministic test data for the settlement
// reconciliation system. It produces 312 transactions, 10 settlement batches,
// and a manifest of 26 planted discrepancies across 8 categories.
//
// Usage:
//
//	go run scripts/generate_test_data.go
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ---------------------------------------------------------------------------
// Mirror types - duplicated from request package so this script runs standalone
// with `go run`. JSON tags are identical to the project's request types.
// ---------------------------------------------------------------------------

type CreateTransactionRequest struct {
	TransactionID       string                 `json:"transaction_id"`
	AuthorizationAmount DecimalStr             `json:"authorization_amount"`
	Currency            string                 `json:"currency"`
	Processor           string                 `json:"processor"`
	Timestamp           time.Time              `json:"timestamp"`
	CustomerCountry     string                 `json:"customer_country"`
	PaymentMethod       string                 `json:"payment_method"`
	Status              string                 `json:"status"`
	MerchantReference   string                 `json:"merchant_reference,omitempty"`
	OriginalCurrency    *string                `json:"original_currency,omitempty"`
	OriginalAmount      *DecimalStr            `json:"original_amount,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

type BatchTransactionRequest struct {
	Transactions []CreateTransactionRequest `json:"transactions"`
}

type FeeItemRequest struct {
	Type        string     `json:"type"`
	Amount      DecimalStr `json:"amount"`
	Description string     `json:"description,omitempty"`
}

type SettlementTransactionRequest struct {
	TransactionID      string           `json:"transaction_id"`
	GrossAmount        DecimalStr       `json:"gross_amount"`
	NetAmount          DecimalStr       `json:"net_amount"`
	Fees               []FeeItemRequest `json:"fees"`
	SettlementDate     time.Time        `json:"settlement_date"`
	ProcessorReference string           `json:"processor_reference,omitempty"`
}

type CreateSettlementRequest struct {
	BatchID           string                         `json:"batch_id"`
	Processor         string                         `json:"processor"`
	SettlementDate    time.Time                      `json:"settlement_date"`
	Currency          string                         `json:"currency"`
	Transactions      []SettlementTransactionRequest `json:"transactions"`
	TotalGross        DecimalStr                     `json:"total_gross"`
	TotalNet          DecimalStr                     `json:"total_net"`
	TotalFees         DecimalStr                     `json:"total_fees"`
	ReserveHoldAmount DecimalStr                     `json:"reserve_hold_amount"`
	ReportGeneratedAt time.Time                      `json:"report_generated_at"`
	Metadata          map[string]interface{}         `json:"metadata,omitempty"`
}

// DecimalStr renders as a JSON string so shopspring/decimal can parse it.
type DecimalStr float64

func (d DecimalStr) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("%.2f", float64(d)))
}

// ---------------------------------------------------------------------------
// Discrepancy manifest types
// ---------------------------------------------------------------------------

type PlantedDiscrepancy struct {
	Type              string   `json:"type"`
	TransactionIDs    []string `json:"transaction_ids"`
	BatchIDs          []string `json:"batch_ids,omitempty"`
	ExpectedImpactUSD float64  `json:"expected_impact_usd"`
	Description       string   `json:"description"`
}

type DiscrepancyManifest struct {
	GeneratedAt        time.Time            `json:"generated_at"`
	Seed               int64                `json:"seed"`
	TotalTransactions  int                  `json:"total_transactions"`
	TotalSettlements   int                  `json:"total_settlements"`
	TotalDiscrepancies int                  `json:"total_discrepancies"`
	TotalImpactUSD     float64              `json:"total_impact_usd"`
	Discrepancies      []PlantedDiscrepancy `json:"discrepancies"`
}

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

const seed int64 = 20260215

var (
	startDate = time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	endDate   = time.Date(2026, 2, 13, 23, 59, 59, 0, time.UTC)

	processorCounts = []struct {
		name  string
		count int
	}{
		{"stripe", 180},
		{"adyen", 80},
		{"dlocal", 52},
	}

	currencies     = []string{"USD", "BRL", "EUR", "MXN"}
	paymentMethods = []string{"credit_card", "debit_card", "pix", "oxxo"}

	countryByCurrency = map[string][]string{
		"USD": {"US", "PR", "EC"},
		"BRL": {"BR"},
		"EUR": {"DE", "FR", "NL", "ES"},
		"MXN": {"MX"},
	}

	// Fee structure: percentage + fixed per txn
	feeRateByProcessor = map[string]float64{
		"stripe": 0.029,
		"adyen":  0.025,
		"dlocal": 0.039,
	}
	feeFixedByProcessor = map[string]float64{
		"stripe": 0.30,
		"adyen":  0.12,
		"dlocal": 0.00,
	}

	fxToUSD = map[string]float64{
		"USD": 1.0,
		"BRL": 0.1835,
		"EUR": 1.0812,
		"MXN": 0.0564,
	}
)

// batchDef describes one settlement batch before transactions are assigned.
type batchDef struct {
	date      time.Time
	processor string
	currency  string
}

// These 10 batches span the processors and currencies, covering the date range.
var batchDefs = []batchDef{
	{time.Date(2026, 1, 22, 12, 0, 0, 0, time.UTC), "stripe", "USD"},
	{time.Date(2026, 1, 27, 12, 0, 0, 0, time.UTC), "stripe", "EUR"},
	{time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC), "stripe", "USD"},
	{time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC), "stripe", "BRL"},
	{time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC), "stripe", "MXN"},
	{time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC), "adyen", "USD"},
	{time.Date(2026, 2, 6, 12, 0, 0, 0, time.UTC), "adyen", "EUR"},
	{time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC), "adyen", "BRL"},
	{time.Date(2026, 1, 30, 12, 0, 0, 0, time.UTC), "dlocal", "MXN"},
	{time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC), "dlocal", "BRL"},
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	rng := rand.New(rand.NewSource(seed))

	// Step 1: Generate 312 transactions.
	transactions := generateTransactions(rng)
	fmt.Printf("Generated %d transactions\n", len(transactions))

	// Step 2: Pre-assign transactions to batches (simulation pass).
	batchAssignment := preAssignBatches(rng, transactions)

	// Step 3: Select discrepancy victims from assigned pools.
	disc := selectVictims(rng, transactions, batchAssignment)

	// Step 4: Build final settlement files with discrepancies applied.
	settlements := buildSettlements(rng, transactions, batchAssignment, disc)

	// Step 5: Build manifest.
	manifest := buildManifest(disc, settlements, transactions)

	// Step 6: Write output files.
	writeJSON(filepath.Join("testdata", "transactions.json"),
		BatchTransactionRequest{Transactions: transactions})

	for _, s := range settlements {
		writeJSON(filepath.Join("testdata", "settlements", s.BatchID+".json"), s)
	}

	writeJSON(filepath.Join("testdata", "planted_discrepancies.json"), manifest)

	fmt.Printf("Generated %d settlement batches with %d planted discrepancies (~$%.0f USD impact)\n",
		len(settlements), manifest.TotalDiscrepancies, manifest.TotalImpactUSD)
	fmt.Println("Done. Files written under testdata/")
}

// ---------------------------------------------------------------------------
// Transaction generation
// ---------------------------------------------------------------------------

func generateTransactions(rng *rand.Rand) []CreateTransactionRequest {
	txns := make([]CreateTransactionRequest, 0, 312)
	idx := 0
	for _, proc := range processorCounts {
		for i := 0; i < proc.count; i++ {
			txns = append(txns, makeTransaction(rng, proc.name, idx))
			idx++
		}
	}
	sort.Slice(txns, func(i, j int) bool {
		return txns[i].Timestamp.Before(txns[j].Timestamp)
	})

	// Ensure exactly 7 recent transactions (last 24h) with AUTHORIZED status.
	recentCutoff := endDate.Add(-24 * time.Hour)
	recentCount := 0
	for i := range txns {
		if txns[i].Timestamp.After(recentCutoff) {
			recentCount++
		}
	}
	// Force the last 7 transactions to be recent and AUTHORIZED.
	if recentCount < 7 {
		for i := len(txns) - 7; i < len(txns); i++ {
			hoursAgo := rng.Float64() * 23.0 // 0-23 hours ago
			txns[i].Timestamp = endDate.Add(-time.Duration(hoursAgo * float64(time.Hour)))
			txns[i].Status = "AUTHORIZED"
		}
	} else {
		// Mark only the last 7 as AUTHORIZED, rest keep their status.
		count := 0
		for i := len(txns) - 1; i >= 0 && count < 7; i-- {
			if txns[i].Timestamp.After(recentCutoff) {
				txns[i].Status = "AUTHORIZED"
				count++
			}
		}
	}

	return txns
}

func makeTransaction(rng *rand.Rand, processor string, idx int) CreateTransactionRequest {
	cur := currencies[rng.Intn(len(currencies))]
	countries := countryByCurrency[cur]
	country := countries[rng.Intn(len(countries))]
	method := paymentMethods[rng.Intn(len(paymentMethods))]

	// Enforce realistic combinations.
	switch method {
	case "pix":
		cur = "BRL"
		country = "BR"
	case "oxxo":
		cur = "MXN"
		country = "MX"
	}

	amount := roundTo2(20.0 + rng.Float64()*4980.0)
	ts := randomTimestamp(rng)

	status := "CAPTURED"
	roll := rng.Float64()
	switch {
	case roll < 0.03:
		status = "DECLINED"
	case roll < 0.05:
		status = "REFUNDED"
	}

	txID := fmt.Sprintf("txn_%s_%04d_%s", processor, idx, shortHash(rng))

	// Merchant reference: marketplace B2B de chocolate format
	seqNum := 1000 + rng.Intn(9000)
	merchantRef := fmt.Sprintf("ORD-CACAO-2026-%04d", seqNum)

	tx := CreateTransactionRequest{
		TransactionID:       txID,
		AuthorizationAmount: DecimalStr(amount),
		Currency:            cur,
		Processor:           processor,
		Timestamp:           ts,
		CustomerCountry:     country,
		PaymentMethod:       method,
		Status:              status,
		MerchantReference:   merchantRef,
	}

	// ~15% of non-USD transactions carry original currency/amount.
	if cur != "USD" && rng.Float64() < 0.15 {
		origCur := "USD"
		origAmt := DecimalStr(roundTo2(amount * fxToUSD[cur]))
		tx.OriginalCurrency = &origCur
		tx.OriginalAmount = &origAmt
	}

	return tx
}

// ---------------------------------------------------------------------------
// Pre-assignment: simulate which transactions land in each batch.
// Returns map[batchIndex] -> list of transaction indices.
// ---------------------------------------------------------------------------

func preAssignBatches(rng *rand.Rand, txns []CreateTransactionRequest) map[int][]int {
	assigned := map[string]bool{} // txnID -> already in a batch
	result := map[int][]int{}

	for bi, bd := range batchDefs {
		windowStart := bd.date.Add(-5 * 24 * time.Hour)
		windowEnd := bd.date.Add(-2 * 24 * time.Hour)

		// First pass: strict window.
		var candidates []int
		for ti, tx := range txns {
			if tx.Processor != bd.processor || tx.Currency != bd.currency {
				continue
			}
			if tx.Status == "DECLINED" || tx.Status == "AUTHORIZED" {
				continue
			}
			if assigned[tx.TransactionID] {
				continue
			}
			if tx.Timestamp.After(windowStart) && tx.Timestamp.Before(windowEnd) {
				candidates = append(candidates, ti)
			}
		}

		// Widen if needed.
		if len(candidates) < 5 {
			wideStart := bd.date.Add(-14 * 24 * time.Hour)
			wideEnd := bd.date.Add(-1 * 24 * time.Hour)
			for ti, tx := range txns {
				if tx.Processor != bd.processor || tx.Currency != bd.currency {
					continue
				}
				if tx.Status == "DECLINED" || tx.Status == "AUTHORIZED" {
					continue
				}
				if assigned[tx.TransactionID] {
					continue
				}
				if tx.Timestamp.After(wideStart) && tx.Timestamp.Before(wideEnd) {
					found := false
					for _, c := range candidates {
						if c == ti {
							found = true
							break
						}
					}
					if !found {
						candidates = append(candidates, ti)
					}
				}
			}
		}

		// Select ~90%.
		var selected []int
		for _, ti := range candidates {
			if rng.Float64() < 0.90 {
				selected = append(selected, ti)
				assigned[txns[ti].TransactionID] = true
			}
		}
		result[bi] = selected
	}

	return result
}

// ---------------------------------------------------------------------------
// Victim selection - picks from pre-assigned pools to guarantee they appear
// in settlement batches.
// ---------------------------------------------------------------------------

type discrepancyPlan struct {
	// Type 1: Missing Settlement - removed from batches during build.
	missingIDs []string
	missingSet map[string]bool

	// Type 2: Amount Mismatch - net amount gets a delta.
	mismatchIDs    []string
	mismatchSet    map[string]bool
	mismatchDeltas map[string]float64 // delta in local currency

	// Type 3: Duplicate Settlement - appears in 2 batches.
	duplicateIDs    []string
	duplicateSet    map[string]bool
	duplicateTarget map[string]int // txnID -> target batch index for the copy

	// Type 4: Orphaned Settlement - fake txn IDs injected.
	orphanEntries []orphanSpec

	// Type 5: Fee Anomaly - processing fee inflated >5%.
	feeAnomalyIDs    []string
	feeAnomalySet    map[string]bool
	feeAnomalyFactor map[string]float64 // multiplier for each victim

	// Type 6: Currency Conversion Error - gross amount uses bad FX.
	fxErrorIDs    []string
	fxErrorSet    map[string]bool
	fxErrorFactor map[string]float64 // multiplier for each victim

	// Type 7: Reserve Hold - batch-level hold amount.
	reserveHoldBatches map[int]float64 // batchIdx -> hold amount in local currency

	// Type 8: Chargeback w/o Notification - chargeback fee on CAPTURED txn.
	chargebackIDs       []string
	chargebackSet       map[string]bool
	chargebackFeeLocal  map[string]float64 // the fee amount in local currency
}

type orphanSpec struct {
	txnID      string
	grossLocal float64
	batchIdx   int
}

func selectVictims(
	rng *rand.Rand,
	txns []CreateTransactionRequest,
	assignment map[int][]int,
) discrepancyPlan {

	d := discrepancyPlan{
		missingSet:          map[string]bool{},
		mismatchSet:         map[string]bool{},
		mismatchDeltas:      map[string]float64{},
		duplicateSet:        map[string]bool{},
		duplicateTarget:     map[string]int{},
		feeAnomalySet:      map[string]bool{},
		feeAnomalyFactor:   map[string]float64{},
		fxErrorSet:          map[string]bool{},
		fxErrorFactor:       map[string]float64{},
		reserveHoldBatches:  map[int]float64{},
		chargebackSet:       map[string]bool{},
		chargebackFeeLocal:  map[string]float64{},
	}

	// Build a flat pool of (batchIdx, txnIdx) pairs for selection.
	type slot struct {
		batchIdx int
		txnIdx   int
	}
	var pool []slot
	for bi, txnIndices := range assignment {
		for _, ti := range txnIndices {
			pool = append(pool, slot{bi, ti})
		}
	}
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	used := map[string]bool{}
	poolPos := 0

	// pickFromPool returns a slot matching the filter, or nil.
	pickFromPool := func(filter func(slot) bool) *slot {
		for i := poolPos; i < len(pool); i++ {
			s := pool[i]
			txID := txns[s.txnIdx].TransactionID
			if used[txID] {
				continue
			}
			if txns[s.txnIdx].Status != "CAPTURED" {
				continue
			}
			if filter != nil && !filter(s) {
				continue
			}
			used[txID] = true
			pool[i], pool[poolPos] = pool[poolPos], pool[i]
			poolPos++
			return &s
		}
		return nil
	}

	// pickClosestUSD picks the slot whose txn USD value is closest to target.
	pickClosestUSD := func(targetUSD float64, filter func(slot) bool) *slot {
		bestIdx := -1
		bestDiff := math.MaxFloat64
		for i := poolPos; i < len(pool); i++ {
			s := pool[i]
			txID := txns[s.txnIdx].TransactionID
			if used[txID] || txns[s.txnIdx].Status != "CAPTURED" {
				continue
			}
			if filter != nil && !filter(s) {
				continue
			}
			tx := txns[s.txnIdx]
			usd := toUSD(float64(tx.AuthorizationAmount), tx.Currency)
			diff := math.Abs(usd - targetUSD)
			if diff < bestDiff {
				bestDiff = diff
				bestIdx = i
			}
		}
		if bestIdx < 0 {
			return nil
		}
		s := pool[bestIdx]
		used[txns[s.txnIdx].TransactionID] = true
		pool[bestIdx], pool[poolPos] = pool[poolPos], pool[bestIdx]
		poolPos++
		return &s
	}

	// --- Type 1: Missing Settlement (5 txns, ~$12,800 USD) ---
	remaining := 12800.0
	for i := 0; i < 5; i++ {
		target := remaining / float64(5-i)
		aged := func(s slot) bool {
			age := endDate.Sub(txns[s.txnIdx].Timestamp)
			return age >= 5*24*time.Hour && age <= 20*24*time.Hour
		}
		s := pickClosestUSD(target, aged)
		if s == nil {
			s = pickClosestUSD(target, nil)
		}
		if s != nil {
			tx := txns[s.txnIdx]
			d.missingIDs = append(d.missingIDs, tx.TransactionID)
			d.missingSet[tx.TransactionID] = true
			remaining -= toUSD(float64(tx.AuthorizationAmount), tx.Currency)
		}
	}

	// --- Type 2: Amount Mismatch (4 txns, ~$8,500 USD) ---
	remainMM := 8500.0
	for i := 0; i < 4; i++ {
		target := remainMM / float64(4-i)
		s := pickClosestUSD(target, nil)
		if s == nil {
			s = pickFromPool(nil)
		}
		if s != nil {
			tx := txns[s.txnIdx]
			d.mismatchIDs = append(d.mismatchIDs, tx.TransactionID)
			d.mismatchSet[tx.TransactionID] = true
			// Compute delta in local currency that maps to target USD.
			deltaLocal := roundTo2(target / fxToUSD[tx.Currency])
			if rng.Float64() < 0.5 {
				deltaLocal = -deltaLocal
			}
			d.mismatchDeltas[tx.TransactionID] = deltaLocal
			remainMM -= toUSD(math.Abs(deltaLocal), tx.Currency)
		}
	}

	// --- Type 3: Duplicate Settlement (3 txns, ~$7,200 USD) ---
	remainDup := 7200.0
	for i := 0; i < 3; i++ {
		target := remainDup / float64(3-i)
		s := pickClosestUSD(target, nil)
		if s != nil {
			tx := txns[s.txnIdx]
			d.duplicateIDs = append(d.duplicateIDs, tx.TransactionID)
			d.duplicateSet[tx.TransactionID] = true
			targetBatch := (s.batchIdx + 4) % len(batchDefs)
			d.duplicateTarget[tx.TransactionID] = targetBatch
			remainDup -= toUSD(float64(tx.AuthorizationAmount), tx.Currency)
		}
	}

	// --- Type 4: Orphaned Settlement (3 entries, ~$5,600 USD) ---
	remainOrphan := 5600.0
	orphanBatches := []int{0, 1, 2}
	for i := 0; i < 3; i++ {
		target := remainOrphan / float64(3-i)
		bi := orphanBatches[i]
		cur := batchDefs[bi].currency
		grossLocal := roundTo2(target / fxToUSD[cur])
		fakeID := fmt.Sprintf("txn_phantom_%04d_%s", i, shortHash(rng))
		d.orphanEntries = append(d.orphanEntries, orphanSpec{
			txnID:      fakeID,
			grossLocal: grossLocal,
			batchIdx:   bi,
		})
		remainOrphan -= toUSD(grossLocal, cur)
	}

	// --- Type 5: Fee Anomaly (4 txns, ~$4,200 USD) ---
	// We need each victim's inflated fee to produce enough USD impact.
	// Impact per victim = (actualFee - expectedFee) in USD.
	// If we inflate the fee by factor F, impact = gross * feeRate * (F - 1) in local.
	// We want gross * feeRate * (F - 1) * fxRate ~ targetPerVictim.
	// So F = 1 + targetPerVictim / (gross * feeRate * fxRate).
	remainFee := 4200.0
	for i := 0; i < 4; i++ {
		target := remainFee / float64(4-i)
		// Pick USD victims for direct impact, or high-value non-USD.
		usdOnly := func(s slot) bool {
			return txns[s.txnIdx].Currency == "USD"
		}
		s := pickClosestUSD(target/0.03, usdOnly)
		if s == nil {
			s = pickClosestUSD(target/0.03, nil)
		}
		if s == nil {
			s = pickFromPool(nil)
		}
		if s != nil {
			tx := txns[s.txnIdx]
			d.feeAnomalyIDs = append(d.feeAnomalyIDs, tx.TransactionID)
			d.feeAnomalySet[tx.TransactionID] = true
			gross := float64(tx.AuthorizationAmount)
			feeRate := feeRateByProcessor[tx.Processor]
			fxRate := fxToUSD[tx.Currency]
			expectedFeeUSD := gross * feeRate * fxRate
			// Compute factor to match target.
			factor := 1.0 + target/expectedFeeUSD
			if factor < 1.10 {
				factor = 1.10
			}
			if factor > 10.0 {
				factor = 10.0
			}
			d.feeAnomalyFactor[tx.TransactionID] = factor
			actualImpactUSD := gross * feeRate * (factor - 1.0) * fxRate
			remainFee -= roundTo2(actualImpactUSD)
		}
	}

	// --- Type 6: Currency Conversion Error (3 txns, ~$3,800 USD) ---
	// Impact = (grossSettlement - grossExpected) in USD = gross * (factor - 1) * fxRate.
	remainFX := 3800.0
	nonUSD := func(s slot) bool { return txns[s.txnIdx].Currency != "USD" }
	// Prefer EUR transactions since they have a high FX rate (1.08 USD/EUR).
	eurOnly := func(s slot) bool { return txns[s.txnIdx].Currency == "EUR" }
	for i := 0; i < 3; i++ {
		target := remainFX / float64(3-i)
		s := pickClosestUSD(target/0.12, eurOnly)
		if s == nil {
			s = pickClosestUSD(target/0.12, nonUSD)
		}
		if s == nil {
			s = pickFromPool(nonUSD)
		}
		if s != nil {
			tx := txns[s.txnIdx]
			d.fxErrorIDs = append(d.fxErrorIDs, tx.TransactionID)
			d.fxErrorSet[tx.TransactionID] = true
			gross := float64(tx.AuthorizationAmount)
			fxRate := fxToUSD[tx.Currency]
			// factor such that gross * (factor - 1) * fxRate = target
			factor := 1.0 + target/(gross*fxRate)
			if factor < 1.08 {
				factor = 1.08
			}
			if factor > 1.50 {
				factor = 1.50
			}
			d.fxErrorFactor[tx.TransactionID] = factor
			actualImpactUSD := gross * (factor - 1.0) * fxRate
			remainFX -= roundTo2(actualImpactUSD)
		}
	}

	// --- Type 7: Reserve Hold (2 batches, ~$2,900 USD) ---
	rhBatches := []int{2, 7}
	rhUSD := []float64{1450.0, 1450.0}
	for i, bi := range rhBatches {
		cur := batchDefs[bi].currency
		localAmt := roundTo2(rhUSD[i] / fxToUSD[cur])
		d.reserveHoldBatches[bi] = localAmt
	}

	// --- Type 8: Chargeback w/o Notification (2 txns, ~$2,000 USD) ---
	// Impact is the full transaction amount (disputed payment that still shows CAPTURED).
	remainCB := 2000.0
	for i := 0; i < 2; i++ {
		target := remainCB / float64(2-i)
		s := pickClosestUSD(target, nil)
		if s == nil {
			s = pickFromPool(nil)
		}
		if s != nil {
			tx := txns[s.txnIdx]
			d.chargebackIDs = append(d.chargebackIDs, tx.TransactionID)
			d.chargebackSet[tx.TransactionID] = true
			// Chargeback fee is a percentage of the transaction (simulating the disputed amount).
			// Use the full transaction amount as impact for the manifest.
			cbFeeLocal := roundTo2(float64(tx.AuthorizationAmount) * 0.15) // 15% dispute fee
			d.chargebackFeeLocal[tx.TransactionID] = cbFeeLocal
			usd := toUSD(float64(tx.AuthorizationAmount), tx.Currency)
			remainCB -= usd
		}
	}

	return d
}

// ---------------------------------------------------------------------------
// Settlement builder
// ---------------------------------------------------------------------------

func buildSettlements(
	rng *rand.Rand,
	txns []CreateTransactionRequest,
	assignment map[int][]int,
	disc discrepancyPlan,
) []CreateSettlementRequest {

	txByID := map[string]*CreateTransactionRequest{}
	for i := range txns {
		txByID[txns[i].TransactionID] = &txns[i]
	}

	settlements := make([]CreateSettlementRequest, len(batchDefs))

	for bi, bd := range batchDefs {
		batchID := fmt.Sprintf("batch_%03d", bi+1)
		var stxns []SettlementTransactionRequest

		for _, ti := range assignment[bi] {
			tx := &txns[ti]

			// Type 1: Missing Settlement - skip these entirely.
			if disc.missingSet[tx.TransactionID] {
				continue
			}

			stxn := makeSettlementTxn(rng, tx, bd, disc)
			stxns = append(stxns, stxn)
		}

		// Type 4: Orphaned entries.
		for _, oe := range disc.orphanEntries {
			if oe.batchIdx == bi {
				proc := bd.processor
				feeRate := feeRateByProcessor[proc]
				fixedFee := feeFixedByProcessor[proc]
				fee := roundTo2(oe.grossLocal*feeRate + fixedFee)
				net := roundTo2(oe.grossLocal - fee)
				stxns = append(stxns, SettlementTransactionRequest{
					TransactionID: oe.txnID,
					GrossAmount:   DecimalStr(oe.grossLocal),
					NetAmount:     DecimalStr(net),
					Fees: []FeeItemRequest{{
						Type:        "processing_fee",
						Amount:      DecimalStr(fee),
						Description: fmt.Sprintf("%s processing fee", proc),
					}},
					SettlementDate:     bd.date,
					ProcessorReference: fmt.Sprintf("ORPHAN-%s", shortHash(rng)),
				})
			}
		}

		// Reserve hold.
		reserveHold := 0.0
		if rh, ok := disc.reserveHoldBatches[bi]; ok {
			reserveHold = rh
		}

		// Calculate totals.
		var totalGross, totalNet, totalFees float64
		for _, st := range stxns {
			totalGross += float64(st.GrossAmount)
			totalNet += float64(st.NetAmount)
			for _, f := range st.Fees {
				totalFees += float64(f.Amount)
			}
		}

		settlements[bi] = CreateSettlementRequest{
			BatchID:           batchID,
			Processor:         bd.processor,
			SettlementDate:    bd.date,
			Currency:          bd.currency,
			Transactions:      stxns,
			TotalGross:        DecimalStr(roundTo2(totalGross)),
			TotalNet:          DecimalStr(roundTo2(totalNet)),
			TotalFees:         DecimalStr(roundTo2(totalFees)),
			ReserveHoldAmount: DecimalStr(roundTo2(reserveHold)),
			ReportGeneratedAt: bd.date.Add(6 * time.Hour),
			Metadata: map[string]interface{}{
				"report_version": "2.1",
				"source":         fmt.Sprintf("%s-settlement-api", bd.processor),
			},
		}
	}

	// Type 3: Duplicate Settlement - copy txn into a second batch.
	for _, dupID := range disc.duplicateIDs {
		targetBI := disc.duplicateTarget[dupID]
		tx := txByID[dupID]
		if tx == nil {
			continue
		}

		// Find the original settlement entry from the source batch.
		var srcEntry *SettlementTransactionRequest
		for si := range settlements {
			for ei := range settlements[si].Transactions {
				if settlements[si].Transactions[ei].TransactionID == dupID {
					srcEntry = &settlements[si].Transactions[ei]
					break
				}
			}
			if srcEntry != nil {
				break
			}
		}

		if srcEntry == nil {
			stxn := makeSettlementTxn(rng, tx, batchDefs[targetBI], disc)
			stxn.ProcessorReference = fmt.Sprintf("DUP-%s", shortHash(rng))
			settlements[targetBI].Transactions = append(settlements[targetBI].Transactions, stxn)
		} else {
			dupEntry := *srcEntry
			dupEntry.ProcessorReference = fmt.Sprintf("DUP-%s", shortHash(rng))
			dupEntry.SettlementDate = batchDefs[targetBI].date
			settlements[targetBI].Transactions = append(settlements[targetBI].Transactions, dupEntry)
		}
	}

	// Recalculate totals for all batches.
	for i := range settlements {
		recalcTotals(&settlements[i])
	}

	return settlements
}

func makeSettlementTxn(
	rng *rand.Rand,
	tx *CreateTransactionRequest,
	bd batchDef,
	disc discrepancyPlan,
) SettlementTransactionRequest {

	gross := float64(tx.AuthorizationAmount)
	proc := tx.Processor
	feeRate := feeRateByProcessor[proc]
	fixedFee := feeFixedByProcessor[proc]

	// Type 6: Currency Conversion Error - inflate gross by computed factor.
	if disc.fxErrorSet[tx.TransactionID] {
		factor := disc.fxErrorFactor[tx.TransactionID]
		gross = roundTo2(gross * factor)
	}

	fee := roundTo2(gross*feeRate + fixedFee)

	// Type 5: Fee Anomaly - inflate fee by computed factor.
	if disc.feeAnomalySet[tx.TransactionID] {
		factor := disc.feeAnomalyFactor[tx.TransactionID]
		fee = roundTo2((gross*feeRate + fixedFee) * factor)
	}

	fees := []FeeItemRequest{{
		Type:        "processing_fee",
		Amount:      DecimalStr(fee),
		Description: fmt.Sprintf("%s processing fee", proc),
	}}
	totalFee := fee

	// Type 8: Chargeback w/o Notification - add chargeback fee.
	if disc.chargebackSet[tx.TransactionID] {
		cbFee := disc.chargebackFeeLocal[tx.TransactionID]
		fees = append(fees, FeeItemRequest{
			Type:        "chargeback_fee",
			Amount:      DecimalStr(cbFee),
			Description: "chargeback processing fee - disputed transaction",
		})
		totalFee += cbFee
	}

	net := roundTo2(gross - totalFee)

	// Type 2: Amount Mismatch - apply delta to net.
	if disc.mismatchSet[tx.TransactionID] {
		delta := disc.mismatchDeltas[tx.TransactionID]
		net = roundTo2(net + delta)
	}

	return SettlementTransactionRequest{
		TransactionID:      tx.TransactionID,
		GrossAmount:        DecimalStr(gross),
		NetAmount:          DecimalStr(net),
		Fees:               fees,
		SettlementDate:     bd.date,
		ProcessorReference: fmt.Sprintf("%s-ref-%s", proc, shortHash(rng)),
	}
}

func recalcTotals(s *CreateSettlementRequest) {
	var tg, tn, tf float64
	for _, st := range s.Transactions {
		tg += float64(st.GrossAmount)
		tn += float64(st.NetAmount)
		for _, f := range st.Fees {
			tf += float64(f.Amount)
		}
	}
	s.TotalGross = DecimalStr(roundTo2(tg))
	s.TotalNet = DecimalStr(roundTo2(tn))
	s.TotalFees = DecimalStr(roundTo2(tf))
}

// ---------------------------------------------------------------------------
// Manifest builder
// ---------------------------------------------------------------------------

func buildManifest(
	disc discrepancyPlan,
	settlements []CreateSettlementRequest,
	txns []CreateTransactionRequest,
) DiscrepancyManifest {

	txByID := map[string]*CreateTransactionRequest{}
	for i := range txns {
		txByID[txns[i].TransactionID] = &txns[i]
	}

	// Helper: find batch IDs containing a given txn ID.
	batchesContaining := func(txnID string) []string {
		var result []string
		for _, s := range settlements {
			for _, st := range s.Transactions {
				if st.TransactionID == txnID {
					result = appendUnique(result, s.BatchID)
					break
				}
			}
		}
		return result
	}

	var discList []PlantedDiscrepancy

	// 1. Missing Settlement
	missImpact := 0.0
	for _, id := range disc.missingIDs {
		if tx := txByID[id]; tx != nil {
			missImpact += toUSD(float64(tx.AuthorizationAmount), tx.Currency)
		}
	}
	discList = append(discList, PlantedDiscrepancy{
		Type:              "missing_settlement",
		TransactionIDs:    disc.missingIDs,
		ExpectedImpactUSD: roundTo2(missImpact),
		Description:       "5 transactions aged 5-20 days excluded from all settlement files",
	})

	// 2. Amount Mismatch
	mmImpact := 0.0
	var mmBatches []string
	for _, id := range disc.mismatchIDs {
		delta := disc.mismatchDeltas[id]
		tx := txByID[id]
		if tx != nil {
			mmImpact += toUSD(math.Abs(delta), tx.Currency)
		}
		mmBatches = appendAllUnique(mmBatches, batchesContaining(id))
	}
	discList = append(discList, PlantedDiscrepancy{
		Type:              "amount_mismatch",
		TransactionIDs:    disc.mismatchIDs,
		BatchIDs:          mmBatches,
		ExpectedImpactUSD: roundTo2(mmImpact),
		Description:       "4 settlements with corrupted net amounts",
	})

	// 3. Duplicate Settlement
	dupImpact := 0.0
	var dupBatches []string
	for _, id := range disc.duplicateIDs {
		if tx := txByID[id]; tx != nil {
			dupImpact += toUSD(float64(tx.AuthorizationAmount), tx.Currency)
		}
		dupBatches = appendAllUnique(dupBatches, batchesContaining(id))
	}
	discList = append(discList, PlantedDiscrepancy{
		Type:              "duplicate_settlement",
		TransactionIDs:    disc.duplicateIDs,
		BatchIDs:          dupBatches,
		ExpectedImpactUSD: roundTo2(dupImpact),
		Description:       "3 transactions appearing in 2 different settlement batches",
	})

	// 4. Orphaned Settlement
	orphanImpact := 0.0
	var orphanIDs []string
	var orphanBatches []string
	for _, oe := range disc.orphanEntries {
		orphanIDs = append(orphanIDs, oe.txnID)
		cur := batchDefs[oe.batchIdx].currency
		orphanImpact += toUSD(oe.grossLocal, cur)
		batchID := fmt.Sprintf("batch_%03d", oe.batchIdx+1)
		orphanBatches = appendUnique(orphanBatches, batchID)
	}
	discList = append(discList, PlantedDiscrepancy{
		Type:              "orphaned_settlement",
		TransactionIDs:    orphanIDs,
		BatchIDs:          orphanBatches,
		ExpectedImpactUSD: roundTo2(orphanImpact),
		Description:       "3 settlement entries referencing transaction IDs that do not exist",
	})

	// 5. Fee Anomaly
	feeImpact := 0.0
	var feeBatches []string
	for _, id := range disc.feeAnomalyIDs {
		tx := txByID[id]
		if tx == nil {
			continue
		}
		for _, s := range settlements {
			for _, st := range s.Transactions {
				if st.TransactionID == id {
					gross := float64(tx.AuthorizationAmount)
					expectedFee := gross*feeRateByProcessor[tx.Processor] + feeFixedByProcessor[tx.Processor]
					var actualFee float64
					for _, f := range st.Fees {
						if f.Type == "processing_fee" {
							actualFee = float64(f.Amount)
						}
					}
					diff := math.Abs(actualFee - expectedFee)
					feeImpact += toUSD(diff, s.Currency)
					feeBatches = appendUnique(feeBatches, s.BatchID)
				}
			}
		}
	}
	discList = append(discList, PlantedDiscrepancy{
		Type:              "fee_anomaly",
		TransactionIDs:    disc.feeAnomalyIDs,
		BatchIDs:          feeBatches,
		ExpectedImpactUSD: roundTo2(feeImpact),
		Description:       "4 transactions with processing fees >5% variance from expected rate",
	})

	// 6. Currency Conversion Error
	fxImpact := 0.0
	var fxBatches []string
	for _, id := range disc.fxErrorIDs {
		tx := txByID[id]
		if tx == nil {
			continue
		}
		for _, s := range settlements {
			for _, st := range s.Transactions {
				if st.TransactionID == id {
					expectedGross := float64(tx.AuthorizationAmount)
					actualGross := float64(st.GrossAmount)
					diff := math.Abs(actualGross - expectedGross)
					fxImpact += toUSD(diff, s.Currency)
					fxBatches = appendUnique(fxBatches, s.BatchID)
				}
			}
		}
	}
	discList = append(discList, PlantedDiscrepancy{
		Type:              "currency_conversion_error",
		TransactionIDs:    disc.fxErrorIDs,
		BatchIDs:          fxBatches,
		ExpectedImpactUSD: roundTo2(fxImpact),
		Description:       "3 transactions with inflated FX rate error applied to gross amount",
	})

	// 7. Reserve Hold
	reserveImpact := 0.0
	var reserveBatches []string
	for bi, localAmt := range disc.reserveHoldBatches {
		cur := batchDefs[bi].currency
		reserveImpact += toUSD(localAmt, cur)
		reserveBatches = append(reserveBatches, settlements[bi].BatchID)
	}
	discList = append(discList, PlantedDiscrepancy{
		Type:              "reserve_hold",
		TransactionIDs:    []string{},
		BatchIDs:          reserveBatches,
		ExpectedImpactUSD: roundTo2(reserveImpact),
		Description:       "2 batches with unexpected reserve_hold_amount reducing payout",
	})

	// 8. Chargeback w/o Notification
	// Impact is the full transaction amount (the disputed payment that should
	// have been flagged as CHARGEDBACK but still shows CAPTURED).
	cbImpact := 0.0
	var cbBatches []string
	for _, id := range disc.chargebackIDs {
		if tx := txByID[id]; tx != nil {
			cbImpact += toUSD(float64(tx.AuthorizationAmount), tx.Currency)
		}
		for _, s := range settlements {
			for _, st := range s.Transactions {
				if st.TransactionID == id {
					cbBatches = appendUnique(cbBatches, s.BatchID)
				}
			}
		}
	}
	discList = append(discList, PlantedDiscrepancy{
		Type:              "chargeback_without_notification",
		TransactionIDs:    disc.chargebackIDs,
		BatchIDs:          cbBatches,
		ExpectedImpactUSD: roundTo2(cbImpact),
		Description:       "2 settlements with chargeback fees but transaction status remains CAPTURED",
	})

	// Aggregate totals.
	totalImpact := 0.0
	totalCount := 0
	for _, d := range discList {
		totalImpact += d.ExpectedImpactUSD
		n := len(d.TransactionIDs)
		if n == 0 {
			n = len(d.BatchIDs)
		}
		totalCount += n
	}

	return DiscrepancyManifest{
		GeneratedAt:        time.Now().UTC(),
		Seed:               seed,
		TotalTransactions:  len(txns),
		TotalSettlements:   len(settlements),
		TotalDiscrepancies: totalCount,
		TotalImpactUSD:     roundTo2(totalImpact),
		Discrepancies:      discList,
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func randomTimestamp(rng *rand.Rand) time.Time {
	rangeSec := endDate.Unix() - startDate.Unix()
	offset := rng.Int63n(rangeSec)
	ts := startDate.Add(time.Duration(offset) * time.Second)
	// Business-hours bias: clamp to 6-22 UTC 80% of the time.
	if rng.Float64() < 0.80 {
		h := ts.Hour()
		if h < 6 {
			ts = ts.Add(time.Duration(6-h) * time.Hour)
		} else if h > 22 {
			ts = ts.Add(-time.Duration(h-22) * time.Hour)
		}
	}
	return ts
}

func shortHash(rng *rand.Rand) string {
	const chars = "abcdef0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rng.Intn(len(chars))]
	}
	return string(b)
}

func toUSD(amount float64, currency string) float64 {
	if rate, ok := fxToUSD[currency]; ok {
		return roundTo2(amount * rate)
	}
	return amount
}

func roundTo2(v float64) float64 {
	return math.Round(v*100) / 100
}

func appendUnique(slice []string, val string) []string {
	for _, v := range slice {
		if v == val {
			return slice
		}
	}
	return append(slice, val)
}

func appendAllUnique(slice []string, vals []string) []string {
	for _, v := range vals {
		slice = appendUnique(slice, v)
	}
	return slice
}

func writeJSON(path string, v interface{}) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", dir, err)
		os.Exit(1)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal %s: %v\n", path, err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("  Wrote %s (%d bytes)\n", path, len(data))
}
