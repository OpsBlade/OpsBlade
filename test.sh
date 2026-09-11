#!/usr/bin/env bash
# Copyright (c) 2025-2026 Tenebris Technologies Inc.
# This software is licensed under the MIT License (see LICENSE for details).
#
# Full regression suite for OpsBlade. Exits 0 only if every section passes.
# Use -x to keep the test log for debugging.

set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR" || exit 1

PRESERVE=false
while getopts "x" opt; do
    case $opt in
        x) PRESERVE=true ;;
        *) echo "Use: $0 [-x]"; exit 2 ;;
    esac
done

LOG="$(mktemp "${TMPDIR:-/tmp}/opsblade-test.XXXXXX")"
cleanup() {
    if [ "$PRESERVE" = true ]; then
        echo "Test log preserved: $LOG"
    else
        rm -f "$LOG"
    fi
}
trap cleanup EXIT

FAILED_SECTIONS=0

section() {
    echo
    echo "==> $1"
}

section "go build ./..."
if ! go build ./...; then
    echo "FAIL: build"
    FAILED_SECTIONS=$((FAILED_SECTIONS + 1))
fi

section "go vet ./..."
if ! go vet ./...; then
    echo "FAIL: vet"
    FAILED_SECTIONS=$((FAILED_SECTIONS + 1))
fi

section "go test -race -count=1 ./..."
go test -race -count=1 -v ./... 2>&1 | tee "$LOG"
TEST_STATUS=${PIPESTATUS[0]}
if [ "$TEST_STATUS" -ne 0 ]; then
    echo "FAIL: go test exited $TEST_STATUS"
    FAILED_SECTIONS=$((FAILED_SECTIONS + 1))
fi

PASSED=$(grep -c -- '^\s*--- PASS' "$LOG" || true)
FAILED=$(grep -c -- '^\s*--- FAIL' "$LOG" || true)
SKIPPED=$(grep -c -- '^\s*--- SKIP' "$LOG" || true)
TOTAL=$((PASSED + FAILED + SKIPPED))

echo
echo "=============================="
echo "Total:   $TOTAL"
echo "Passed:  $PASSED"
echo "Failed:  $FAILED"
echo "Skipped: $SKIPPED"
echo "=============================="

if [ "$TOTAL" -eq 0 ]; then
    echo "FAIL: no tests ran"
    FAILED_SECTIONS=$((FAILED_SECTIONS + 1))
fi

if [ "$FAILED_SECTIONS" -ne 0 ] || [ "$FAILED" -ne 0 ]; then
    echo "RESULT: FAIL"
    exit 1
fi
echo "RESULT: PASS"
exit 0
