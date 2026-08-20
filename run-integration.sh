#!/usr/bin/env sh
# run-integration.sh — Run integration tests against OT&E / sandbox endpoints.
#
# Credentials come ONLY from environment variables — never hardcode them here.
# Per-registrar credentials use the EPP_REGISTRARS_<NAME>_<FIELD> convention:
#
#   EPP_REGISTRARS_INTERNETX_HOST
#   EPP_REGISTRARS_INTERNETX_USERNAME
#   EPP_REGISTRARS_INTERNETX_PASSWORD
#   EPP_REGISTRARS_NICAT_HOST
#   EPP_REGISTRARS_NICAT_USERNAME
#   EPP_REGISTRARS_NICAT_PASSWORD
#   EPP_REGISTRARS_EURID_HOST
#   EPP_REGISTRARS_EURID_USERNAME
#   EPP_REGISTRARS_EURID_PASSWORD
#   EPP_REGISTRARS_DENIC_HOST
#   EPP_REGISTRARS_DENIC_USERNAME
#   EPP_REGISTRARS_DENIC_PASSWORD
#
# Optional environment variables honored by the test harness:
#   TEST_REPORT_DIR     — report output directory (default: ./reports/)
#   TEST_REPORT_FORMATS — comma-separated formats: junit,json,text,html (default: all)
#
# RUN_ID: unique prefix for test object names (CICD-04).
# Prevents collisions when multiple CI jobs run against the same OT&E sandbox.
# If RUN_ID is not set, defaults to a Unix timestamp so each run is unique.
: "${RUN_ID:=$(date +%s)}"
export RUN_ID

# Pass extra go test flags via "$@" (e.g., -run TestInternetX, -timeout 120s).
set -e

exec go test -tags integration -v ./... "$@"
