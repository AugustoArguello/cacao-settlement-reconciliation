#!/usr/bin/env bash
# Retrieve a reconciliation report and query discrepancies.
# Pass REPORT_ID as an env variable, or the script will fetch the latest one.

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"

# If no REPORT_ID provided, fetch the first one from the list
if [ -z "${REPORT_ID:-}" ]; then
  echo "No REPORT_ID provided. Fetching from report list..."
  REPORT_ID=$(curl -s "${BASE_URL}/api/v1/reconciliation/reports" | jq -r '.[0].report_id // empty')
  if [ -z "$REPORT_ID" ]; then
    echo "Error: No reports found. Run 03-run-reconciliation.sh first."
    exit 1
  fi
  echo "Using report: ${REPORT_ID}"
  echo ""
fi

echo "=== Full Report ==="
echo ""
curl -s "${BASE_URL}/api/v1/reconciliation/reports/${REPORT_ID}" | jq .

echo ""
echo "=== Discrepancies: Critical severity ==="
echo ""
curl -s "${BASE_URL}/api/v1/reconciliation/discrepancies?severity=critical" | jq '{
  total_count: .total_count,
  items: [.data[] | {type, severity, transaction_id, difference, description}]
}'

echo ""
echo "=== Discrepancies: By type (MISSING_SETTLEMENT) ==="
echo ""
curl -s "${BASE_URL}/api/v1/reconciliation/discrepancies?type=MISSING_SETTLEMENT" | jq '{
  total_count: .total_count,
  items: [.data[] | {transaction_id, difference, description}]
}'

echo ""
echo "=== Discrepancies: By processor (stripe) ==="
echo ""
curl -s "${BASE_URL}/api/v1/reconciliation/discrepancies?processor=stripe&page_size=5" | jq '{
  total_count: .total_count,
  items: [.data[:5][] | {type, transaction_id, difference}]
}'

echo ""
echo "Done."
