#!/usr/bin/env bash
# =============================================================================
# test_siem.sh — SIEM stack health check for llama-chat.my.id
#
# Checks:
#   1. Prometheus   — http(s)://HOST:9090/-/healthy
#   2. Grafana      — http(s)://HOST:3001/api/health
#   3. Loki         — http(s)://HOST:3100/ready
#   4. Alertmanager — http(s)://HOST:9093/-/healthy  (optional)
#   5. Node Export  — http(s)://HOST:9100/metrics     (optional)
#
# Usage:
#   ./test_siem.sh                        # uses defaults below
#   ./test_siem.sh 23.100.94.231          # custom host
#   SIEM_HOST=llama-chat.my.id ./test_siem.sh
#   GRAFANA_PASS=secret ./test_siem.sh    # test Grafana with auth
# =============================================================================

set -euo pipefail

# ── Config (override via env vars or first arg) ──────────────────────────────
HOST="${1:-${SIEM_HOST:-llama-chat.my.id}}"
SCHEME="${SIEM_SCHEME:-https}"            # http or https
TIMEOUT="${SIEM_TIMEOUT:-10}"            # seconds per request
GRAFANA_USER="${GRAFANA_USER:-admin}"
GRAFANA_PASS="${GRAFANA_PASS:-}"         # leave empty to skip auth test

# Ports (adjust if your setup uses different ones)
PROMETHEUS_PORT="${PROMETHEUS_PORT:-9090}"
GRAFANA_PORT="${GRAFANA_PORT:-3001}"
LOKI_PORT="${LOKI_PORT:-3100}"
ALERTMANAGER_PORT="${ALERTMANAGER_PORT:-9093}"
NODE_EXPORTER_PORT="${NODE_EXPORTER_PORT:-9100}"

# ── Colors ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ── State ────────────────────────────────────────────────────────────────────
PASS=0
FAIL=0
SKIP=0

# ── Helpers ──────────────────────────────────────────────────────────────────
print_header() {
    echo ""
    echo -e "${BOLD}${CYAN}══════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}${CYAN}  SIEM Stack Health Check — ${HOST}${NC}"
    echo -e "${BOLD}${CYAN}══════════════════════════════════════════════════${NC}"
    echo ""
}

ok()   { echo -e "  ${GREEN}✔  PASS${NC}  $*"; ((PASS++)); }
fail() { echo -e "  ${RED}✘  FAIL${NC}  $*"; ((FAIL++)); }
skip() { echo -e "  ${YELLOW}⊘  SKIP${NC}  $*"; ((SKIP++)); }
info() { echo -e "  ${CYAN}ℹ${NC}       $*"; }
section() { echo -e "\n${BOLD}▶ $*${NC}"; }

# Generic HTTP check
# Usage: check <label> <url> [expected_http_code] [body_pattern] [extra_curl_args...]
check() {
    local label="$1"
    local url="$2"
    local expected_code="${3:-200}"
    local body_pattern="${4:-}"
    shift 4 || true          # remaining args passed to curl

    local http_code body
    # -k to tolerate self-signed certs (common on VPS)
    body=$(curl -sk --max-time "${TIMEOUT}" -o /tmp/siem_body -w "%{http_code}" \
           "$@" "${url}" 2>/dev/null) || { fail "${label} — curl error (host unreachable?)"; return; }
    body=$(cat /tmp/siem_body 2>/dev/null || echo "")

    if [[ "${http_code}" != "${expected_code}" ]]; then
        fail "${label} — expected HTTP ${expected_code}, got ${http_code}"
        [[ -n "${body}" ]] && info "Response body: ${body:0:200}"
        return
    fi

    if [[ -n "${body_pattern}" ]] && ! echo "${body}" | grep -qiE "${body_pattern}"; then
        fail "${label} — HTTP ${http_code} but body pattern '${body_pattern}' not found"
        info "Response body: ${body:0:200}"
        return
    fi

    ok "${label} — HTTP ${http_code}${body_pattern:+ (pattern matched)}"
}

# ── Main ─────────────────────────────────────────────────────────────────────
print_header
info "Target  : ${SCHEME}://${HOST}"
info "Timeout : ${TIMEOUT}s per request"
info "Date    : $(date)"

# ── 1. Prometheus ─────────────────────────────────────────────────────────────
section "Prometheus  (port ${PROMETHEUS_PORT})"
PROM_BASE="${SCHEME}://${HOST}:${PROMETHEUS_PORT}"

check "Readiness endpoint"      "${PROM_BASE}/-/healthy"  200 "Prometheus Server is Healthy"
check "Ready endpoint"          "${PROM_BASE}/-/ready"    200 "Prometheus Server is Ready"
check "Metrics API reachable"   "${PROM_BASE}/api/v1/query?query=up" 200 '"status":"success"'

# Spot-check: is the app's backend being scraped?
SCRAPE_BODY=$(curl -sk --max-time "${TIMEOUT}" \
    "${PROM_BASE}/api/v1/query?query=up" 2>/dev/null || echo "")
if echo "${SCRAPE_BODY}" | grep -qiE '"__name__":"up"'; then
    ok "Scrape targets present (up metric exists)"
else
    fail "No scrape targets found — check prometheus.yml scrape_configs"
fi

# ── 2. Grafana ────────────────────────────────────────────────────────────────
section "Grafana  (port ${GRAFANA_PORT})"
GF_BASE="${SCHEME}://${HOST}:${GRAFANA_PORT}"

check "Health API"              "${GF_BASE}/api/health"   200 '"database":"ok"'
check "Login page reachable"    "${GF_BASE}/login"        200 "grafana|login"

if [[ -n "${GRAFANA_PASS}" ]]; then
    info "Testing Grafana login with user '${GRAFANA_USER}'..."
    check "Grafana API auth" \
        "${GF_BASE}/api/org" 200 '"id"' \
        -u "${GRAFANA_USER}:${GRAFANA_PASS}"

    # Check at least one datasource is configured
    DS_BODY=$(curl -sk --max-time "${TIMEOUT}" \
        -u "${GRAFANA_USER}:${GRAFANA_PASS}" \
        "${GF_BASE}/api/datasources" 2>/dev/null || echo "[]")
    if echo "${DS_BODY}" | grep -qiE '"type"'; then
        ok "Datasource(s) configured in Grafana"
        info "Datasources: $(echo "${DS_BODY}" | grep -oP '"name":"[^"]+"' | tr '\n' ' ')"
    else
        fail "No datasources found in Grafana — connect Prometheus/Loki"
    fi

    # Check at least one dashboard exists
    DASH_BODY=$(curl -sk --max-time "${TIMEOUT}" \
        -u "${GRAFANA_USER}:${GRAFANA_PASS}" \
        "${GF_BASE}/api/search?type=dash-db" 2>/dev/null || echo "[]")
    DASH_COUNT=$(echo "${DASH_BODY}" | grep -c '"type":"dash-db"' || true)
    if [[ "${DASH_COUNT}" -gt 0 ]]; then
        ok "Dashboards found: ${DASH_COUNT}"
    else
        fail "No dashboards found in Grafana"
    fi
else
    skip "Grafana auth/datasource test — set GRAFANA_PASS to enable"
fi

# ── 3. Loki ───────────────────────────────────────────────────────────────────
section "Loki  (port ${LOKI_PORT})"
LOKI_BASE="${SCHEME}://${HOST}:${LOKI_PORT}"

check "Ready endpoint"          "${LOKI_BASE}/ready"      200 "ready"
check "Metrics endpoint"        "${LOKI_BASE}/metrics"    200 "loki_"

# Check Loki has received some logs
LOKI_LABELS=$(curl -sk --max-time "${TIMEOUT}" \
    "${LOKI_BASE}/loki/api/v1/labels" 2>/dev/null || echo "")
if echo "${LOKI_LABELS}" | grep -qiE '"status":"success"'; then
    ok "Loki labels API responding"
    LABEL_LIST=$(echo "${LOKI_LABELS}" | grep -oP '"[^"]+"' | grep -v '"status"' | grep -v '"success"' | grep -v '"data"' | tr '\n' ' ')
    [[ -n "${LABEL_LIST}" ]] && info "Available labels: ${LABEL_LIST}"
else
    fail "Loki labels API not responding correctly"
fi

# ── 4. Alertmanager (optional) ───────────────────────────────────────────────
section "Alertmanager  (port ${ALERTMANAGER_PORT})  [optional]"
AM_BASE="${SCHEME}://${HOST}:${ALERTMANAGER_PORT}"

AM_STATUS=$(curl -sk --max-time "${TIMEOUT}" -o /dev/null -w "%{http_code}" \
    "${AM_BASE}/-/healthy" 2>/dev/null || echo "000")
if [[ "${AM_STATUS}" == "200" ]]; then
    ok "Alertmanager healthy"
    check "Alertmanager API" "${AM_BASE}/api/v2/status" 200 '"status"'
else
    skip "Alertmanager not reachable on port ${ALERTMANAGER_PORT} (HTTP ${AM_STATUS}) — skipping"
fi

# ── 5. Node Exporter (optional) ──────────────────────────────────────────────
section "Node Exporter  (port ${NODE_EXPORTER_PORT})  [optional]"
NE_BASE="${SCHEME}://${HOST}:${NODE_EXPORTER_PORT}"

NE_STATUS=$(curl -sk --max-time "${TIMEOUT}" -o /dev/null -w "%{http_code}" \
    "${NE_BASE}/metrics" 2>/dev/null || echo "000")
if [[ "${NE_STATUS}" == "200" ]]; then
    ok "Node Exporter metrics endpoint reachable"
    NE_BODY=$(curl -sk --max-time "${TIMEOUT}" "${NE_BASE}/metrics" 2>/dev/null | head -5)
    info "Sample: ${NE_BODY:0:120}"
else
    skip "Node Exporter not reachable on port ${NODE_EXPORTER_PORT} (HTTP ${NE_STATUS}) — skipping"
fi

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}${CYAN}══════════════════════════════════════════════════${NC}"
echo -e "${BOLD}  Results: ${GREEN}${PASS} passed${NC}  ${RED}${FAIL} failed${NC}  ${YELLOW}${SKIP} skipped${NC}"
echo -e "${BOLD}${CYAN}══════════════════════════════════════════════════${NC}"
echo ""

if [[ "${FAIL}" -gt 0 ]]; then
    echo -e "${RED}${BOLD}⚠  ${FAIL} check(s) failed. Review the output above.${NC}"
    echo ""
    exit 1
else
    echo -e "${GREEN}${BOLD}✔  All required checks passed.${NC}"
    echo ""
    exit 0
fi
