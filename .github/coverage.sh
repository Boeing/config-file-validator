#!/bin/bash

set -e

COVERAGE_THRESHOLD=95

# Run unit tests and produce a coverage report
go test -cover -coverprofile coverage.out ./...

# Validate that the coverage is above or at the required threshold
echo "Checking if test coverage is above threshold ..."
echo "Coverage threshold: $COVERAGE_THRESHOLD %"
totalCoverage=`go tool cover -func coverage.out | grep total | grep -Eo '[0-9]+\.[0-9]+'`
echo "Current test coverage : $totalCoverage %"
if (( $(echo "$COVERAGE_THRESHOLD <= $totalCoverage" | bc -l) )); then
  echo "Coverage OK"
else
  echo "Current test coverage is below threshold"
  exit 1
fi