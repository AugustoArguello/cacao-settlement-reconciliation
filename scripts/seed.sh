#!/usr/bin/env bash
# seed.sh - Load test data into the settlement reconciliation API.
#
# Usage:
#   ./scripts/seed.sh [BASE_URL]
#
# Defaults to http://localhost:8080 if no BASE_URL is provided.
# Assumes the server is already running.

set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
API="${BASE_URL}/api/v1"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
TESTDATA_DIR="${PROJECT_DIR}/testdata"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo " Cacao Settlement Reconciliation - Seeder"
echo "=========================================="
echo ""
echo "API Base URL: ${API}"
echo "Test data dir: ${TESTDATA_DIR}"
echo ""

# Health check
echo -n "Checking API health... "
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/health" 2>/dev/null || true)
if [ "$HTTP_CODE" != "200" ]; then
    echo -e "${RED}FAILED${NC} (HTTP ${HTTP_CODE:-timeout})"
    echo "Make sure the server is running at ${BASE_URL}"
    exit 1
fi
echo -e "${GREEN}OK${NC}"
echo ""

# Step 1: Load transactions in batch
echo "--- Step 1: Loading transactions ---"
if [ ! -f "${TESTDATA_DIR}/transactions.json" ]; then
    echo -e "${RED}ERROR: ${TESTDATA_DIR}/transactions.json not found${NC}"
    echo "Run 'go run scripts/generate_test_data.go' first to generate test data."
    exit 1
fi

echo -n "  POST /api/v1/transactions/batch ... "
HTTP_CODE=$(curl -s -o /tmp/seed_txn_response.json -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d @"${TESTDATA_DIR}/transactions.json" \
    "${API}/transactions/batch")

if [ "$HTTP_CODE" -ge 200 ] && [ "$HTTP_CODE" -lt 300 ]; then
    echo -e "${GREEN}${HTTP_CODE} OK${NC}"
else
    echo -e "${RED}${HTTP_CODE} FAILED${NC}"
    cat /tmp/seed_txn_response.json 2>/dev/null
    echo ""
fi

# Step 2: Load settlement batches one by one
echo ""
echo "--- Step 2: Loading settlement batches ---"
SETTLEMENT_DIR="${TESTDATA_DIR}/settlements"
if [ ! -d "${SETTLEMENT_DIR}" ]; then
    echo -e "${RED}ERROR: ${SETTLEMENT_DIR} not found${NC}"
    exit 1
fi

SETTLEMENT_COUNT=0
SETTLEMENT_ERRORS=0
for settlement_file in "${SETTLEMENT_DIR}"/batch_*.json; do
    filename=$(basename "$settlement_file")
    echo -n "  POST /api/v1/settlements ($filename) ... "

    HTTP_CODE=$(curl -s -o /tmp/seed_stl_response.json -w "%{http_code}" \
        -X POST \
        -H "Content-Type: application/json" \
        -d @"${settlement_file}" \
        "${API}/settlements")

    if [ "$HTTP_CODE" -ge 200 ] && [ "$HTTP_CODE" -lt 300 ]; then
        echo -e "${GREEN}${HTTP_CODE} OK${NC}"
        SETTLEMENT_COUNT=$((SETTLEMENT_COUNT + 1))
    else
        echo -e "${RED}${HTTP_CODE} FAILED${NC}"
        cat /tmp/seed_stl_response.json 2>/dev/null
        echo ""
        SETTLEMENT_ERRORS=$((SETTLEMENT_ERRORS + 1))
    fi
done

# Summary
echo ""
echo "=========================================="
echo " Seed Summary"
echo "=========================================="
echo "  Transactions batch: loaded"
echo "  Settlement batches: ${SETTLEMENT_COUNT} loaded, ${SETTLEMENT_ERRORS} errors"
echo ""

if [ -f "${TESTDATA_DIR}/planted_discrepancies.json" ]; then
    DISC_COUNT=$(python3 -c "import json; d=json.load(open('${TESTDATA_DIR}/planted_discrepancies.json')); print(d['total_discrepancies'])" 2>/dev/null || echo "?")
    DISC_IMPACT=$(python3 -c "import json; d=json.load(open('${TESTDATA_DIR}/planted_discrepancies.json')); print(f\"\${d['total_impact_usd']:,.2f}\")" 2>/dev/null || echo "?")
    echo -e "${YELLOW}Planted discrepancies: ${DISC_COUNT} (~\$${DISC_IMPACT} USD impact)${NC}"
    echo "  See testdata/planted_discrepancies.json for the full manifest."
fi

echo ""
echo "You can now run reconciliation:"
echo "  curl -X POST ${API}/reconciliation/run"
echo ""

# Cleanup temp files
rm -f /tmp/seed_txn_response.json /tmp/seed_stl_response.json
