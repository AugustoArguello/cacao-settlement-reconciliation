#!/usr/bin/env bash
# Upload all 10 settlement batches to the reconciliation API.
# Each batch represents a processor settlement report (Stripe, Adyen, dLocal).

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"

echo "=== Uploading 10 settlement batches ==="
echo ""

for f in testdata/settlements/batch_*.json; do
  batch_id=$(basename "$f" .json)
  echo "  Uploading ${batch_id}..."
  curl -s -X POST "${BASE_URL}/api/v1/settlements" \
    -H "Content-Type: application/json" \
    -d @"$f" | jq -c '{batch_id: .batch_id, processor: .processor, txn_count: (.transactions | length)}'
done

echo ""
echo "=== Verifying: list all settlements ==="
echo ""

curl -s "${BASE_URL}/api/v1/settlements?page=1&page_size=10" | jq '{total_count: .total_count, page: .page}'

echo ""
echo "Done. Settlement batches loaded."
