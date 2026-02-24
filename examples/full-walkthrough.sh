#!/usr/bin/env bash
# Full end-to-end walkthrough of the settlement reconciliation API.
# Runs all steps: health check -> upload data -> reconcile -> inspect results.

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"

echo "============================================================"
echo "  Cacao Direct - Settlement Reconciliation: Full Walkthrough"
echo "============================================================"
echo ""

# Step 0: Health check
echo "--- Step 0: Health Check ---"
echo ""
curl -s "${BASE_URL}/health" | jq .
echo ""

# Step 1: Upload transactions
echo "--- Step 1: Upload 312 Transactions ---"
echo ""
curl -s -X POST "${BASE_URL}/api/v1/transactions/batch" \
  -H "Content-Type: application/json" \
  -d @testdata/transactions.json | jq .
echo ""

# Step 2: Upload settlements
echo "--- Step 2: Upload 10 Settlement Batches ---"
echo ""
for f in testdata/settlements/batch_*.json; do
  batch_id=$(basename "$f" .json)
  result=$(curl -s -X POST "${BASE_URL}/api/v1/settlements" \
    -H "Content-Type: application/json" \
    -d @"$f")
  echo "  ${batch_id}: $(echo "$result" | jq -c '{processor: .processor, txns: (.transactions | length)}')"
done
echo ""

# Step 3: Run reconciliation
echo "--- Step 3: Run Reconciliation Engine ---"
echo ""
REPORT=$(curl -s -X POST "${BASE_URL}/api/v1/reconciliation/run" \
  -H "Content-Type: application/json" \
  -d '{
    "period_start": "2026-01-15T00:00:00Z",
    "period_end": "2026-01-31T23:59:59Z",
    "include_temporal_analysis": true
  }')

echo "$REPORT" | jq '{
  report_id: .report_id,
  status: .status,
  total_transactions: .total_transactions,
  total_settlements: .total_settlements,
  net_discrepancy: .net_discrepancy,
  discrepancy_percentage: .discrepancy_percentage,
  avg_settlement_days: .avg_settlement_days
}'
echo ""

REPORT_ID=$(echo "$REPORT" | jq -r '.report_id')

# Step 4: Discrepancy breakdown
echo "--- Step 4: Discrepancy Breakdown ---"
echo ""
echo "$REPORT" | jq '[.discrepancy_breakdown[] | {type, count, total_amount}]'
echo ""

# Step 5: Processor summary
echo "--- Step 5: Processor Summary ---"
echo ""
echo "$REPORT" | jq '[.processor_summary[] | {processor, transaction_count, net_discrepancy, discrepancy_count}]'
echo ""

# Step 6: Critical discrepancies
echo "--- Step 6: Critical Discrepancies ---"
echo ""
curl -s "${BASE_URL}/api/v1/reconciliation/discrepancies?severity=critical" | jq '{
  total_critical: .total_count,
  items: [.data[] | {type, transaction_id, difference, description}]
}'
echo ""

# Step 7: Slow settlements
echo "--- Step 7: Slow Settlements (>10 days) ---"
echo ""
echo "$REPORT" | jq '[.slow_settlements[]? | {transaction_id, days_to_settle, processor, amount}]'
echo ""

echo "============================================================"
echo "  Walkthrough complete."
echo "  Report ID: ${REPORT_ID}"
echo "  View full report:"
echo "    curl -s ${BASE_URL}/api/v1/reconciliation/reports/${REPORT_ID} | jq ."
echo "============================================================"
