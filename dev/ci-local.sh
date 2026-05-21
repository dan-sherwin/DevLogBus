#!/usr/bin/env bash
set -euo pipefail
trap "rm -f coverage.out" EXIT

command -v golangci-lint >/dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
command -v govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
command -v npm >/dev/null 2>&1 || {
  echo "npm is required to build the embedded devlogbusd UI"
  exit 1
}

export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.26.3}"

npm --prefix internal/devlogbusd/ui ci
npm --prefix internal/devlogbusd/ui run build

go mod tidy
go build ./...
go vet ./...
go test ./... -race -count=1 -covermode=atomic -coverprofile=coverage.out
go tool cover -func=coverage.out | awk -v thr="${COVER_THRESH:-0}" '
/^total:/ {
  gsub(/%/, "", $3);
  cov = $3 + 0;
  if (cov < thr) {
    printf "FAIL: coverage %.1f%% < %d%%\n", cov, thr;
    exit 1
  }
  exit 0
}
END {
  if (NR == 0) {
    print "ERROR: no coverage data.";
    exit 2
  }
}'
rm -f coverage.out

GOGC=off golangci-lint config verify
GOGC=off golangci-lint run --timeout 5m
govulncheck -test ./...

test -z "$(gofmt -s -l .)" || {
  echo "gofmt needed"
  exit 1
}
