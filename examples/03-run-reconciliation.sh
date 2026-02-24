#!/usr/bin/env bash
# Run the reconciliation engine over the loaded data.
# Covers Jan 15 - Jan 31, 2026 with temporal analysis enabled.

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"

echo "=== Running reconciliation engine ==="
echo ""

REPORT=$(curl -s -X POST "${BASE_URL}/api/v1/reconciliation/run" \
  -H "Content-Type: application/json" \
  -d '{
    "period_start": "2026-01-15T00:00:00Z",
    "period_end": "2026-01-31T23:59:59Z",
    "include_temporal_analysis": true
  }')

# Print summary
echo "$REPORT" | jq '{
  report_id: .report_id,
  status: .status,
  total_transactions: .total_transactions,
  total_settlements: .total_settlements,
  net_discrepancy: .net_discrepancy,
  discrepancy_percentage: .discrepancy_percentage,
  avg_settlement_days: .avg_settlement_days,
  discrepancy_breakdown: [.discrepancy_breakdown[] | {type, count, total_amount}],
  slow_settlements_count: (.slow_settlements | length)
}'

echo ""

# Save report_id for the next script
REPORT_ID=$(echo "$REPORT" | jq -r '.report_id')
echo "Report ID: ${REPORT_ID}"
echo ""
echo "Done. Reconciliation complete."
echo "Use this report ID with 04-get-report.sh:"
echo "  REPORT_ID=${REPORT_ID} ./examples/04-get-report.sh"
