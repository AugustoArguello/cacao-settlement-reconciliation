# Design Document

## Data Flow

```
                                 INGESTION PHASE
  ============================================================================

  External Sources                    API Layer                   Storage
  ----------------                    ---------                   -------

  Transaction Logs ──> POST /transactions/batch ──> TransactionService
  (PSP webhooks,       BatchTransactionRequest        .CreateBatch()
   internal ledger)         |                              |
                            |  Validate each               |
                            |  - transaction_id required   v
                            |  - amount > 0            +----------+
                            |  - valid currency        | In-Memory|
                            |  - valid status          | Txn Map  |
                            |  - 2-letter country      | (by ID)  |
                            +------------------------->+----------+

  Settlement Files ──> POST /settlements (per batch) ──> SettlementService
  (Stripe CSV,         CreateSettlementRequest             .Create()
   Adyen SFTP,              |                                  |
   dLocal API)              |  Validate                        |
                            |  - batch_id required             v
                            |  - processor required       +-----------+
                            |  - valid currency           | In-Memory |
                            |  - settlement_date          | Stl Map   |
                            +---->                        | (by batch)|
                                                          +-----------+

                              RECONCILIATION PHASE
  ============================================================================

  POST /reconciliation/run
  RunReconciliationRequest
    { period_start, period_end, include_temporal_analysis }
         |
         v
  +-------------------------------+
  | ReconciliationService         |
  | .RunReconciliation()          |
  |                               |
  | 1. Fetch txns in period       |
  | 2. Fetch settlements in period|
  | 3. Fetch fee rules            |
  |                               |
  | 4. Run 8 detectors:           |
  |    +------------------------+ |     +-------------------+
  |    | detectMissingSettle.   |------>|                   |
  |    | detectAmountMismatch   |------>|   Discrepancy     |
  |    | detectDuplicateSettle. |------>|   Store           |
  |    | detectOrphanedSettle.  |------>|   (in-memory)     |
  |    | detectFeeAnomalies     |------>|                   |
  |    | detectCurrencyConv.    |------>+-------------------+
  |    | detectReserveHolds     |
  |    | detectChargebackNotif. |
  |    +------------------------+ |
  |                               |
  | 5. Build report               |     +-------------------+
  | 6. Optional temporal analysis |---->|   Report Store    |
  | 7. Store report               |     |   (in-memory)     |
  +-------------------------------+     +-------------------+
         |
         v
  ReconciliationReport (JSON)


                                QUERY PHASE
  ============================================================================

  GET /reconciliation/reports          --> List all reports
  GET /reconciliation/reports/:id      --> Single report with full breakdown
  GET /reconciliation/discrepancies    --> Paginated, filterable discrepancy list
      ?type=MISSING_SETTLEMENT
      &severity=critical
      &processor=stripe
      &page=1&page_size=50
```

## Why Batch Over Streaming

Settlement reconciliation is fundamentally a batch problem. Here is why:

1. **Settlements arrive in batches.** Processors like Stripe, Adyen, and dLocal produce settlement reports on a daily or weekly cadence. You get a CSV/JSON with all transactions settled in that period. The data isn't a stream -- it's a complete snapshot.

2. **Reconciliation requires both sides.** To detect a missing settlement, you need the full set of transactions AND the full set of settlements for a period. Streaming one side while waiting for the other creates complexity with windowing, late arrivals, and partial state.

3. **Correctness over latency.** A reconciliation run that takes 200ms and processes the complete dataset is more valuable than a streaming pipeline that produces partial results. Financial discrepancies need to be accurate, not fast.

4. **Operational simplicity.** A batch API is easy to trigger from a cron job, easy to debug (replay the same request), and easy to audit. Streaming adds Kafka/SQS, consumer groups, offset management, and exactly-once semantics.

Where streaming fits: real-time fraud detection on individual transactions. That's a different system. The reconciliation engine sits downstream and runs periodically.

## Algorithm Complexity Analysis

All 8 detectors operate on the same input: a list of N transactions and M settlement batches containing S total settlement line items.

### Per-Detector Breakdown

| Detector | Time Complexity | Space Complexity | Notes |
|----------|----------------|-----------------|-------|
| Missing Settlement | O(S + N) | O(S) | Build set of settled txn IDs, then scan transactions |
| Amount Mismatch | O(N + S) | O(N) | Build txn lookup map, then scan settlement items |
| Duplicate Settlement | O(S) | O(S) | Build txn-to-batches map, then filter count > 1 |
| Orphaned Settlement | O(N + S) | O(N) | Build known txn ID set, then scan settlement items |
| Fee Anomaly | O(N + S * F) | O(N) | F = avg fees per settlement item (typically 1-3) |
| Currency Conversion | O(N + S) | O(N) | Build txn lookup, check FX rate for cross-currency |
| Reserve Hold | O(M) | O(1) | One pass over settlement batches |
| Chargeback Detection | O(N + S * F) | O(N) | Scan fees for chargeback-related keywords |

### Overall

The full reconciliation run is **O(N + S + M)** with **O(N + S)** space, where:
- N = number of transactions (312 in test data)
- S = total settlement line items across all batches (~289 in test data)
- M = number of settlement batches (10 in test data)

Each detector makes a single pass (or two at most). No nested loops over the full dataset. The txn lookup map is built once and reused across detectors (in practice, each detector builds its own, but they could share one with minor refactoring).

### Report Builder

The report builder is O(N + S + D) where D is the number of discrepancies found. It builds:
- Discrepancy breakdown by type: O(D)
- Processor summary: O(N + S + D) via hash maps
- Temporal analysis (optional): O(S) single pass

For the test dataset (312 txns, 10 batches, 26 discrepancies), the entire reconciliation completes in <5ms on commodity hardware.

## Architecture Decisions

### In-Memory Storage

The `repository/` package defines clean interfaces (`TransactionRepository`, `SettlementRepository`, etc.) with `repository/memory/` providing thread-safe in-memory implementations using `sync.RWMutex` + Go maps.

This gives us:
- **Zero infrastructure** -- `go run` and you're up.
- **Fast test cycles** -- no database setup, no migrations, no fixtures.
- **Clear upgrade path** -- implement the same interfaces with `database/sql` or an ORM, change one line in `routers.go`, done.

The tradeoff is no persistence. In production, you'd use PostgreSQL with the same interfaces.

### Interface-Driven Design

Every service depends on repository interfaces, not concrete implementations:

```go
type ReconciliationService struct {
    txnRepo  repository.TransactionRepository   // interface
    stlRepo  repository.SettlementRepository    // interface
    discRepo repository.DiscrepancyRepository   // interface
    rptRepo  repository.ReportRepository        // interface
    feeRepo  repository.FeeRuleRepository       // interface
}
```

This enables:
- Swapping storage backends without touching business logic.
- Unit testing with mock repositories.
- Clear dependency boundaries between layers.

### Layered Architecture

```
  Controllers (HTTP)  -->  Services (Business Logic)  -->  Repository (Data)
  - Parse request          - Validate                     - Store
  - HTTP status codes      - Run detectors                - Query
  - Error formatting       - Build reports                - Filter
```

Each layer has a single responsibility. Controllers never touch the database. Services never set HTTP status codes. Repositories never validate business rules.

### Decimal Precision

All monetary amounts use `shopspring/decimal` instead of `float64`. This prevents the classic floating-point issue where `0.1 + 0.2 != 0.3`. In a reconciliation system, a rounding error of $0.01 per transaction across 10,000 transactions is a $100 discrepancy -- the exact kind of problem this system is designed to detect.

### Deterministic Test Data

The `scripts/generate_test_data.go` script uses a seeded random number generator (`seed: 20260215`) to produce the same 312 transactions, 10 settlement batches, and 26 planted discrepancies every time. This makes the test data:

- **Reproducible** -- any developer gets identical data.
- **Verifiable** -- the `planted_discrepancies.json` manifest documents exactly what the engine should find.
- **Realistic** -- multiple processors (Stripe, Adyen, dLocal), currencies (USD, BRL, EUR, MXN, COP), payment methods (credit_card, debit_card, pix, oxxo, spei, pse), and edge cases.

### Error Response Consistency

All error responses follow the same structure:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "period_end must be after period_start"
  }
}
```

Error codes: `INVALID_JSON`, `VALIDATION_ERROR`, `DUPLICATE`, `NOT_FOUND`, `INTERNAL_ERROR`.
