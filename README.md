# Cacao Direct - Settlement Reconciliation API

A high-performance settlement reconciliation engine that ingests payment transactions and processor settlement reports, then runs 8 specialized detection algorithms to surface discrepancies -- missing settlements, amount mismatches, duplicate payouts, orphaned entries, fee anomalies, FX conversion errors, reserve holds, and stealth chargebacks.

Built for the Yuno Engineering Challenge. Go + Echo v4. Zero external dependencies beyond the standard library and three lightweight packages.

## Quick Start

```bash
# 1. Run the server
go run cmd/server/main.go

# 2. Run tests
make test

# 3. Health check
curl -s http://localhost:8080/health | jq
```

## Architecture

```
                          Cacao Settlement Reconciliation Engine
  ============================================================================

  Transactions (312)            Settlements (10 batches)
  POST /api/v1/transactions     POST /api/v1/settlements
         |                               |
         v                               v
  +-------------+                +---------------+
  |  In-Memory  |                |   In-Memory   |
  |  Txn Store  |                |   Stl Store   |
  +------+------+                +-------+-------+
         |                               |
         +---------------+---------------+
                         |
                         v
              +---------------------+
              |  Reconciliation     |
              |  Engine             |
              |                     |
              |  8 Detectors:       |
              |  1. Missing Settle. |
              |  2. Amount Mismatch |
              |  3. Duplicates      |       +---------------+
              |  4. Orphaned Settle.|------>|  Discrepancy  |
              |  5. Fee Anomalies   |       |  Store        |
              |  6. FX Conversion   |       +---------------+
              |  7. Reserve Holds   |
              |  8. Chargebacks     |       +---------------+
              |                     |------>|  Report       |
              +---------------------+       |  Store        |
                                            +---------------+
                         |
                         v
              GET /api/v1/reconciliation/reports/:id
              GET /api/v1/reconciliation/discrepancies
```

## API Reference

### Health Check

```bash
curl -s http://localhost:8080/health | jq
```

### Transactions

#### Create Single Transaction

```bash
curl -s -X POST http://localhost:8080/api/v1/transactions \
  -H "Content-Type: application/json" \
  -d '{
    "transaction_id": "txn_stripe_0001",
    "authorization_amount": "2500.00",
    "currency": "USD",
    "processor": "stripe",
    "timestamp": "2026-01-15T10:00:00Z",
    "customer_country": "US",
    "payment_method": "credit_card",
    "status": "CAPTURED",
    "merchant_reference": "ORDER-12345"
  }' | jq
```

**Response** `201 Created`:
```json
{
  "transaction_id": "txn_stripe_0001",
  "authorization_amount": "2500.00",
  "currency": "USD",
  "processor": "stripe",
  "timestamp": "2026-01-15T10:00:00Z",
  "customer_country": "US",
  "payment_method": "credit_card",
  "status": "CAPTURED",
  "merchant_reference": "ORDER-12345"
}
```

#### Batch Upload Transactions

```bash
curl -s -X POST http://localhost:8080/api/v1/transactions/batch \
  -H "Content-Type: application/json" \
  -d @testdata/transactions.json | jq
```

**Response** `201 Created`:
```json
{
  "ingested": 312,
  "duplicates_skipped": 0,
  "errors": []
}
```

#### List Transactions

```bash
# All transactions (paginated)
curl -s "http://localhost:8080/api/v1/transactions?page=1&page_size=10" | jq

# Filter by processor, currency, status, country, date range
curl -s "http://localhost:8080/api/v1/transactions?processor=stripe&currency=USD&status=CAPTURED" | jq
```

**Response** `200 OK`:
```json
{
  "data": [ ... ],
  "total_count": 312,
  "page": 1,
  "page_size": 10
}
```

#### Get Transaction by ID

```bash
curl -s http://localhost:8080/api/v1/transactions/txn_stripe_0001 | jq
```

### Settlements

#### Create Single Settlement

```bash
curl -s -X POST http://localhost:8080/api/v1/settlements \
  -H "Content-Type: application/json" \
  -d '{
    "batch_id": "batch_001",
    "processor": "stripe",
    "settlement_date": "2026-01-22T12:00:00Z",
    "currency": "USD",
    "transactions": [
      {
        "transaction_id": "txn_stripe_0001",
        "gross_amount": "2500.00",
        "net_amount": "2427.50",
        "fees": [
          { "type": "processing_fee", "amount": "72.50" }
        ],
        "settlement_date": "2026-01-22T12:00:00Z"
      }
    ],
    "total_gross": "2500.00",
    "total_net": "2427.50",
    "total_fees": "72.50",
    "reserve_hold_amount": "0.00",
    "report_generated_at": "2026-01-22T18:00:00Z"
  }' | jq
```

#### Batch Upload Settlements

```bash
# Upload all 10 settlement batches
for f in testdata/settlements/batch_*.json; do
  curl -s -X POST http://localhost:8080/api/v1/settlements \
    -H "Content-Type: application/json" \
    -d @"$f" | jq
done
```

#### List Settlements

```bash
curl -s "http://localhost:8080/api/v1/settlements?processor=stripe&currency=USD" | jq
```

#### Get Settlement by Batch ID

```bash
curl -s http://localhost:8080/api/v1/settlements/batch_001 | jq
```

### Reconciliation

#### Run Reconciliation

```bash
curl -s -X POST http://localhost:8080/api/v1/reconciliation/run \
  -H "Content-Type: application/json" \
  -d '{
    "period_start": "2026-01-15T00:00:00Z",
    "period_end": "2026-01-31T23:59:59Z",
    "include_temporal_analysis": true
  }' | jq
```

**Response** `200 OK` (abbreviated):
```json
{
  "report_id": "uuid-here",
  "period_start": "2026-01-15T00:00:00Z",
  "period_end": "2026-01-31T23:59:59Z",
  "total_transactions": 312,
  "total_settlements": 289,
  "total_expected": "...",
  "total_actual": "...",
  "net_discrepancy": "...",
  "discrepancy_percentage": "...",
  "discrepancy_breakdown": [
    {
      "type": "MISSING_SETTLEMENT",
      "count": 5,
      "total_amount": "9775.24",
      "severity_breakdown": { "critical": 3, "warning": 2 }
    }
  ],
  "processor_summary": [ ... ],
  "problematic_transactions": [ ... ],
  "avg_settlement_days": "5.2",
  "slow_settlements": [ ... ],
  "status": "COMPLETED"
}
```

#### List Reports

```bash
curl -s http://localhost:8080/api/v1/reconciliation/reports | jq
```

#### Get Report by ID

```bash
curl -s http://localhost:8080/api/v1/reconciliation/reports/{report_id} | jq
```

#### List Discrepancies

```bash
# All discrepancies (paginated)
curl -s "http://localhost:8080/api/v1/reconciliation/discrepancies" | jq

# Filter by type, severity, processor
curl -s "http://localhost:8080/api/v1/reconciliation/discrepancies?type=MISSING_SETTLEMENT&severity=critical" | jq
curl -s "http://localhost:8080/api/v1/reconciliation/discrepancies?processor=stripe" | jq
```

**Response** `200 OK`:
```json
{
  "data": [
    {
      "id": "uuid",
      "type": "MISSING_SETTLEMENT",
      "severity": "critical",
      "transaction_id": "txn_adyen_0256_77c45b13",
      "expected_amount": "1955.05",
      "actual_amount": "0",
      "difference": "1955.05",
      "currency": "USD",
      "processor": "adyen",
      "description": "Transaction captured 14 days ago but not found in any settlement report",
      "investigation_details": {
        "days_since_capture": 14,
        "action": "Contact processor to inquire about settlement status"
      },
      "detected_at": "2026-01-31T...",
      "resolved": false
    }
  ],
  "total_count": 26,
  "page": 1,
  "page_size": 50
}
```

## Detection Algorithms

The reconciliation engine runs **8 independent detectors**, each targeting a specific class of settlement discrepancy:

| # | Detector | What It Catches | Severity Logic |
|---|----------|----------------|---------------|
| 1 | **Missing Settlement** | Captured transactions with no matching settlement entry after N days | Warning at 3+ days, Critical at 7+ days |
| 2 | **Amount Mismatch** | Net settlement differs from expected (authorized - fees) beyond tolerance | Warning by default, Critical if delta > $100 |
| 3 | **Duplicate Settlement** | Same transaction_id appears in multiple settlement batches | Always Critical |
| 4 | **Orphaned Settlement** | Settlement references a transaction_id not in our system | Warning (may be cross-system) |
| 5 | **Fee Anomaly** | Actual fee rate deviates from expected processor rate by >5% | Warning at 5%, Critical at 20% |
| 6 | **Currency Conversion** | Implied FX rate deviates >3% from market rate, or same-currency gross != authorized | Warning at 3%, Critical at 10% |
| 7 | **Reserve Hold** | Processor withholds funds as reserve, reducing effective payout | Info <10%, Warning 10-25%, Critical >25% |
| 8 | **Chargeback w/o Notification** | Settlement has chargeback/dispute fees but internal status is not CHARGEDBACK | Always Critical |

### Fee Validation

The engine knows default fee schedules for each processor:

| Processor | Rate | Flat Fee |
|-----------|------|----------|
| Stripe | 2.9% | $0.30 |
| Adyen | 2.5% | $0.12 |
| dLocal | 3.9% | $0.00 |
| Default | 3.0% | $0.25 |

Custom fee rules can override these defaults via the `FeeRule` model.

### Amount Mismatch Tolerance

The engine uses a smart tolerance of `max($0.01, 0.5% of transaction amount)` to avoid false positives from legitimate rounding differences while still catching real discrepancies.

## Test Data

The test dataset is deterministic (seed: `20260215`) and contains **26 planted discrepancies** across 312 transactions and 10 settlement batches totaling **~$35K in discrepancy impact**:

| Discrepancy Type | Count | Impact (USD) | Description |
|-----------------|-------|-------------|-------------|
| Missing Settlement | 5 | $9,775.24 | Transactions aged 5-14 days with no settlement |
| Amount Mismatch | 4 | $8,500.00 | Corrupted net amounts in settlements |
| Duplicate Settlement | 3 | $7,276.29 | Same txn in 2 different batches |
| Orphaned Settlement | 3 | $5,600.00 | Settlement refs to non-existent txns |
| Reserve Hold | 2 | $2,900.00 | Unexpected reserve deductions |
| Currency Conversion | 3 | $1,005.37 | 8-15% FX rate errors |
| Fee Anomaly | 4 | $86.85 | Fees >5% off expected rate |
| Chargeback w/o Notice | 2 | $21.35 | Chargeback fees, status still CAPTURED |
| **Total** | **26** | **$35,165.10** | |

Generate test data:
```bash
go run scripts/generate_test_data.go
```

## Assumptions & Trade-offs

| Decision | Rationale | Trade-off |
|----------|-----------|-----------|
| **In-memory storage** | Zero setup, fast iteration, fits challenge scope | No persistence across restarts |
| **Batch over streaming** | Settlements are inherently batch (daily/weekly reports) | Not suited for real-time alerting |
| **Repository interfaces** | Clean separation; swap to PostgreSQL by implementing the interface | Slight over-engineering for a challenge |
| **Hardcoded FX rates** | Demonstrates the detection logic without external API dependency | Rates drift from reality |
| **shopspring/decimal** | Avoids float64 precision errors on financial amounts | Slightly more verbose than float math |
| **No authentication** | Out of scope for the challenge | Would need API keys / JWT in production |
| **Tolerance-based matching** | `max($0.01, 0.5%)` prevents false positives from rounding | May miss very small discrepancies |
| **Deterministic test data** | Seeded RNG makes tests reproducible and verifiable | Less variety than real-world data |

## What I Would Add With More Time

- **PostgreSQL + migrations** -- replace in-memory stores with a real database; the repository interfaces are ready for it.
- **Webhook notifications** -- push alerts for critical discrepancies to Slack, PagerDuty, or email.
- **Streaming ingestion** -- Kafka/SQS consumer for real-time transaction feeds alongside the batch API.
- **Scheduled reconciliation** -- cron-based runs with configurable periods per processor.
- **Dashboard UI** -- React/Next.js frontend showing discrepancy trends, processor health, and drill-down views.
- **Authentication & RBAC** -- API key auth with role-based access (viewer, operator, admin).
- **Rate limiting & request validation middleware** -- protect against abuse and malformed payloads.
- **Idempotent reconciliation runs** -- prevent duplicate reports if the same period is reconciled twice.
- **Historical FX rate API integration** -- pull real market rates from a provider like Open Exchange Rates.
- **Prometheus metrics** -- expose reconciliation duration, discrepancy counts, and ingestion throughput.
- **Containerization** -- Dockerfile + docker-compose with PostgreSQL for one-command local setup.
- **CI/CD pipeline** -- GitHub Actions with lint, test, build, and deploy stages.

## Project Structure

```
.
├── cmd/server/main.go          # Entrypoint
├── config/config.go            # Environment config
├── controllers/                # HTTP handlers (Echo)
│   ├── reconciliation_controller.go
│   ├── settlement_controller.go
│   └── transaction_controller.go
├── models/                     # Domain models
│   ├── discrepancy.go          # 8 discrepancy types + severity
│   ├── fee_rule.go             # Configurable fee rules
│   ├── report.go               # Reconciliation report
│   ├── settlement.go           # Settlement batch model
│   └── transaction.go          # Transaction + enums
├── repository/                 # Repository interfaces
│   ├── interfaces.go           # Contracts for all stores
│   └── memory/                 # In-memory implementations
├── request/                    # Request DTOs + validation
├── response/                   # Response DTOs
├── routers/routers.go          # Route registration
├── services/                   # Business logic
│   ├── reconciliation_service.go  # 8 detectors + report builder
│   ├── settlement_service.go
│   └── transaction_service.go
├── scripts/                    # Data generation
│   └── generate_test_data.go   # Deterministic test data generator
├── testdata/                   # Pre-generated test data
│   ├── transactions.json       # 312 transactions
│   ├── settlements/            # 10 settlement batches
│   └── planted_discrepancies.json  # Expected results manifest
├── examples/                   # Runnable example scripts
├── docs/                       # Design documentation
└── Makefile                    # Build, test, lint commands
```

## Tech Stack

| Component | Choice |
|-----------|--------|
| Language | Go 1.25 |
| HTTP Framework | Echo v4 |
| Decimal Math | shopspring/decimal |
| UUID Generation | google/uuid |
| Storage | In-memory (interface-ready for SQL) |
