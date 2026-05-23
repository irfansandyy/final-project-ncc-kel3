#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# scripts/setup.sh
# Interactive setup: starts the CI/CD stack and walks through configuration
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

BOLD='\033[1m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
RED='\033[0;31m'
NC='\033[0m'

header() { echo -e "\n${CYAN}${BOLD}══════════════════════════════════════════${NC}"; echo -e "${CYAN}${BOLD}  $1${NC}"; echo -e "${CYAN}${BOLD}══════════════════════════════════════════${NC}"; }
step()   { echo -e "${GREEN}▶ $1${NC}"; }
warn()   { echo -e "${YELLOW}⚠  $1${NC}"; }
info()   { echo -e "   $1"; }

header "NCC Chatbot — CI/CD Stack Setup"

# ── Prerequisites check ───────────────────────────────────────────────────────
step "Checking prerequisites..."
for cmd in docker curl; do
    command -v "$cmd" &>/dev/null \
        && info "✓ $cmd found" \
        || { echo -e "${RED}✗ $cmd not found — please install it first${NC}"; exit 1; }
done

# Verify docker compose plugin (v2)
docker compose version &>/dev/null \
    && info "✓ docker compose (v2) found" \
    || { echo -e "${RED}✗ 'docker compose' plugin not found — please install Docker Compose v2${NC}"; exit 1; }

# ── Kernel parameter required by Elasticsearch (SonarQube) ───────────────────
CURRENT_VM_MAP=$(sysctl -n vm.max_map_count 2>/dev/null || echo 0)
if [ "$CURRENT_VM_MAP" -lt 262144 ]; then
    warn "Setting vm.max_map_count=262144 (required by SonarQube/ES)"
    sudo sysctl -w vm.max_map_count=262144
    echo "vm.max_map_count=262144" | sudo tee -a /etc/sysctl.conf > /dev/null
    info "✓ vm.max_map_count set"
fi

# ── Start services ────────────────────────────────────────────────────────────
header "Starting Docker services"
cd "$(dirname "$0")/.."

step "Building Jenkins image..."
docker compose build jenkins

step "Starting all services (jenkins, sonarqube, postgres, dind)..."
docker compose up -d

step "Waiting for Jenkins to be ready (this may take ~60s)..."
until curl -sf http://localhost:8080/login > /dev/null 2>&1; do
    echo -n "."; sleep 5
done; echo ""
info "✓ Jenkins is up at http://localhost:8080"

step "Waiting for SonarQube to be ready (this may take ~2 min)..."
until curl -sf http://localhost:9000/api/system/status \
      | grep -q '"status":"UP"' 2>/dev/null; do
    echo -n "."; sleep 8
done; echo ""
info "✓ SonarQube is up at http://localhost:9000"

# ── Jenkins initial password ───────────────────────────────────────────────────
header "Jenkins Initial Setup"
INIT_PWD=$(docker exec jenkins-blueocean \
    cat /var/jenkins_home/secrets/initialAdminPassword 2>/dev/null || echo "NOT_FOUND")

if [ "$INIT_PWD" != "NOT_FOUND" ]; then
    echo -e "${YELLOW}Initial Admin Password:${NC} ${BOLD}${INIT_PWD}${NC}"
else
    warn "Could not read initial password — Jenkins may already be configured."
fi

# ── Next steps guide ───────────────────────────────────────────────────────────
header "Manual Configuration Steps"

echo -e "${BOLD}1. SONARQUBE SETUP  →  http://localhost:9000${NC}"
info "   Login: admin / admin  (change password on first login)"
info "   a) Administration → Security → Users → admin → Generate Token"
info "      Name: jenkins-token  →  copy the token"
info "   b) Administration → Configuration → Webhooks → Create"
info "      Name: Jenkins | URL: http://jenkins-blueocean:8080/sonarqube-webhook/"
echo ""

echo -e "${BOLD}2. JENKINS SETUP  →  http://localhost:8080${NC}"
info "   a) Manage Jenkins → Plugins → Install:"
info "      • SonarQube Scanner    • GitHub    • Blue Ocean"
info "   b) Manage Jenkins → Credentials → (global) → Add:"
info "      • Secret text  ID: SONAR_HOST_URL    Value: http://sonarqube:9000"
info "      • Secret text  ID: DISCORD_WEBHOOK_URL  Value: <your Discord webhook>"
info "      • Secret text  ID: SONAR_TOKEN       Value: <token from step 1a>"
info "      • Username/password  ID: github-credentials"
info "   c) Manage Jenkins → System:"
info "      SonarQube servers → Add:"
info "        Name: SonarQube | URL: http://sonarqube:9000"
info "        Auth token: SONAR_TOKEN credential"
info "   d) Manage Jenkins → Tools:"
info "      SonarQube Scanner → Add → Name: SonarScanner (install automatically)"
echo ""

echo -e "${BOLD}3. GITHUB WEBHOOK  →  your repo settings${NC}"
info "   Payload URL:  http://<YOUR_SERVER_IP>:8080/github-webhook/"
info "   Content type: application/json"
info "   Events:       Just the push event  (+ Pull requests optional)"
echo ""

echo -e "${BOLD}4. CREATE JENKINS PIPELINE JOB${NC}"
info "   New Item → Pipeline → Name: ncc-chatbot"
info "   Pipeline → Definition: Pipeline script from SCM"
info "   SCM: Git | URL: https://github.com/irfansandyy/final-project-ncc-kel3"
info "   Credentials: github-credentials"
info "   Branch: */E1"
info "   Script Path: Jenkinsfile"
info "   ☑ GitHub hook trigger for GITScm polling"
echo ""

echo -e "${BOLD}5. DISCORD WEBHOOK${NC}"
info "   Discord server → Channel settings → Integrations → Create Webhook"
info "   Copy URL → add as DISCORD_WEBHOOK_URL credential in Jenkins"
echo ""

header "Files to copy to your repository root"
echo "  Jenkinsfile              → pipeline definition"
echo "  sonar-project.properties → SonarQube config"
echo ""
echo -e "${GREEN}${BOLD}Setup complete! Visit http://localhost:8080 to continue.${NC}"
