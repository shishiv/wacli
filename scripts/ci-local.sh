#!/usr/bin/env bash
# Standalone Local / Contabo CI Runner for wacli
# Replicates the full GitHub Actions CI matrix without any GitHub Actions dependency.
set -euo pipefail

BOLD="\033[1m"
GREEN="\033[32m"
RED="\033[31m"
BLUE="\033[34m"
RESET="\033[0m"

log_step() {
  echo -e "\n${BOLD}${BLUE}==> [CI] $1${RESET}"
}

log_ok() {
  echo -e "${GREEN}✔ $1${RESET}"
}

log_err() {
  echo -e "${RED}✖ $1${RESET}" >&2
}

start_time=$(date +%s)

log_step "Checking toolchain prerequisites"
for cmd in go pnpm node gcc git; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    log_err "Missing prerequisite: $cmd"
    exit 1
  fi
done
echo "Go: $(go version)"
echo "Node: $(node --version)"
echo "pnpm: $(pnpm --version)"
echo "GCC: $(gcc --version | head -n 1)"
log_ok "Toolchain present"

log_step "Step 1/8: Git whitespace & diff hygiene"
git diff --check
log_ok "Git hygiene check passed"

log_step "Step 2/8: Format check (gofmt)"
pnpm --silent format:check
log_ok "Format check passed"

log_step "Step 3/8: Go linter (vet)"
pnpm --silent lint
log_ok "Linter passed"

log_step "Step 4/8: Standard library security vulnerabilities (govulncheck)"
pnpm --silent govulncheck:source
log_ok "Govulncheck passed"

log_step "Step 5/8: Deadcode detection"
pnpm --silent lint:deadcode
log_ok "Zero deadcode"

log_step "Step 6/8: Test suite (plain Go, FTS5, Windows lock, CGO assertion, doc tests)"
CGO_ENABLED=1 pnpm --silent test
log_ok "Test suite passed"

log_step "Step 7/8: E2E Store & SQLC verification"
bash scripts/e2e-store-sqlc.sh
log_ok "E2E store verification passed"

log_step "Step 8/8: Production build (sqlite_fts5) and docs site"
CGO_ENABLED=1 pnpm --silent build
pnpm --silent docs:site
log_ok "Production build compiled: dist/wacli; docs site built"

end_time=$(date +%s)
duration=$((end_time - start_time))

echo -e "\n${BOLD}${GREEN}==========================================="
echo -e "🎉 FULL CI GATE PASSED in ${duration}s"
echo -e "===========================================${RESET}"
