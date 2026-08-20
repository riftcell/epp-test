#!/usr/bin/env sh
# run-unit.sh — Run offline unit tests against mock servers.
#
# Unit tests are fully self-contained and require no external network access.
# All tests under ./... that carry the `unit` build tag are executed.
#
# Optional environment variables honored by the test harness:
#   TEST_REPORT_DIR     — report output directory (default: ./reports/)
#   TEST_REPORT_FORMATS — comma-separated formats: junit,json,text,html (default: all)
#
# Credentials: not required for unit tests — mock servers handle all I/O locally.
# Pass extra go test flags via "$@" (e.g., -run TestInternetX, -count=1).
set -e

exec go test -tags unit -v ./... "$@"
