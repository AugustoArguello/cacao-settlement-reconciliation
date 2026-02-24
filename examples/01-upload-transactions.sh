#!/usr/bin/env bash
# Upload transactions to the reconciliation API.
# Uses the pre-generated test dataset (312 transactions across 3 processors).

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"

echo "=== Uploading 312 transactions (batch) ==="
echo ""

curl -s -X POST "${BASE_URL}/api/v1/transactions/batch" \
  -H "Content-Type: application/json" \
  -d @testdata/transactions.json | jq .

echo ""
echo "=== Verifying: list first 5 transactions ==="
echo ""

curl -s "${BASE_URL}/api/v1/transactions?page=1&page_size=5" | jq .

echo ""
echo "Done. Transactions loaded."
