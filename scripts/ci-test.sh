#!/usr/bin/env bash
# Runs the Go tests in CI. On failure the failing lines are also written as
# one annotation, which shows on the run page without signing in.
set -o pipefail
go test ./internal/... 2>&1 | tee test.log
status=$?
if [ $status -ne 0 ]; then
  msg=$(grep -E -- '--- FAIL|_test\.go:[0-9]+|panic:|^FAIL|\.go:[0-9]+:[0-9]+:' test.log | head -60 | sed 's/%/%25/g' | awk '{printf "%s%%0A", $0}')
  echo "::error title=Tes gagal::${msg}"
fi
exit $status
