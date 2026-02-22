#!/usr/bin/env bash
#
# Dirigent installer
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/ercadev/dirigent-releases/main/install.sh | sudo bash
#   -- or --
#   sudo bash install.sh
#
# To pin a specific version:
#   DIRIGENT_VERSION=v0.2.0 curl -fsSL ... | sudo bash
#
# Supported operating systems:
#   Ubuntu 22.04 (Jammy) and later
#   Debian 11 (Bullseye) and later
#
# The script must be run as root or via sudo. It will exit immediately
# with a descriptive message if that condition is not met.
#
# Each phase of the install is announced with a step() call so you can
# follow progress without needing verbose output from individual commands.

set -euo pipefail

# ─── output helpers ───────────────────────────────────────────────────────────

# step prints a human-readable progress line so the user can see what the
# installer is doing at any point. Keep messages short and imperative.
step() {
    echo "  -->  $*"
}

# error prints a message to stderr and exits with status 1. Call this
# whenever a condition makes it impossible to continue safely.
error() {
    echo "error: $*" >&2
    exit 1
}

# ─── pre-flight: root check ───────────────────────────────────────────────────

step "Checking for root privileges"

if [ "$(id -u)" -ne 0 ]; then
    error "This script must be run as root or with sudo.
       Re-run with: sudo bash install.sh"
fi

# ─── pre-flight: OS detection ─────────────────────────────────────────────────

step "Detecting operating system"

if [ ! -f /etc/os-release ]; then
    error "Cannot determine OS: /etc/os-release not found.
       Dirigent supports Ubuntu 22.04+ and Debian 11+."
fi

# Source the file to get ID and VERSION_ID as shell variables.
# shellcheck source=/dev/null
. /etc/os-release

OS_ID="${ID:-unknown}"
OS_VERSION_ID="${VERSION_ID:-0}"
OS_MAJOR="${OS_VERSION_ID%%.*}"   # e.g. "22" from "22.04", "11" from "11"

case "${OS_ID}" in
    ubuntu)
        OS_MAJOR_INT="${OS_VERSION_ID%%.*}"
        if [ "${OS_MAJOR_INT}" -lt 22 ]; then
            error "Ubuntu ${OS_VERSION_ID} is not supported.
       Minimum required version: Ubuntu 22.04 (Jammy)."
        fi
        ;;
    debian)
        if [ "${OS_MAJOR}" -lt 11 ]; then
            error "Debian ${OS_VERSION_ID} is not supported.
       Minimum required version: Debian 11 (Bullseye)."
        fi
        ;;
    *)
        error "Unsupported operating system: ${OS_ID}.
       Dirigent supports Ubuntu 22.04+ and Debian 11+."
        ;;
esac

step "Detected ${PRETTY_NAME:-${OS_ID} ${OS_VERSION_ID}}"

# ─── pre-flight: architecture detection ──────────────────────────────────────

ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *) error "Unsupported architecture: ${ARCH}. Supported: x86_64, aarch64." ;;
esac

step "Detected architecture: ${ARCH}"

# ─── version resolution ───────────────────────────────────────────────────────

DIRIGENT_VERSION="${DIRIGENT_VERSION:-latest}"

if [ "${DIRIGENT_VERSION}" = "latest" ]; then
    RELEASE_BASE="https://github.com/ercadev/dirigent-releases/releases/latest/download"
else
    RELEASE_BASE="https://github.com/ercadev/dirigent-releases/releases/download/${DIRIGENT_VERSION}"
fi

step "Using release: ${DIRIGENT_VERSION}"

# ─── Docker installation ──────────────────────────────────────────────────────

# install_docker installs Docker Engine via the official apt repository for the
# detected OS. It assumes apt-get is available and OS_ID / VERSION_CODENAME are
# set from /etc/os-release.
install_docker() {
    step "Installing prerequisites"
    apt-get install -y -q ca-certificates curl gnupg

    step "Adding Docker's official GPG key"
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL "https://download.docker.com/linux/${OS_ID}/gpg" \
        | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg

    step "Adding Docker apt repository"
    echo \
        "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
https://download.docker.com/linux/${OS_ID} ${VERSION_CODENAME} stable" \
        | tee /etc/apt/sources.list.d/docker.list > /dev/null

    step "Updating apt package index"
    apt-get update -qq

    step "Installing Docker Engine"
    apt-get install -y -q \
        docker-ce docker-ce-cli containerd.io \
        docker-buildx-plugin docker-compose-plugin
}

step "Checking for existing Docker installation"

if command -v docker > /dev/null 2>&1; then
    step "Docker already installed ($(docker --version | head -1)); skipping"
    STEP_DOCKER="already installed"
else
    install_docker
    step "Docker installed ($(docker --version | head -1))"
    STEP_DOCKER="installed"
fi

# ─── Bun installation ─────────────────────────────────────────────────────────

# install_bun downloads the Bun runtime binary directly from GitHub releases
# and places it at /usr/local/bin/bun, making it available system-wide.
install_bun() {
    # Bun uses different arch names than Go.
    local bun_arch
    case "${ARCH}" in
        amd64) bun_arch="x64" ;;
        arm64) bun_arch="aarch64" ;;
    esac

    local bun_tmp
    bun_tmp=$(mktemp -d)

    step "Installing unzip (required to extract Bun)"
    apt-get install -y -q unzip

    step "Downloading Bun runtime (linux/${ARCH})"
    curl -fsSL \
        "https://github.com/oven-sh/bun/releases/latest/download/bun-linux-${bun_arch}.zip" \
        -o "${bun_tmp}/bun.zip"

    unzip -q "${bun_tmp}/bun.zip" -d "${bun_tmp}"
    install -m 755 "${bun_tmp}/bun-linux-${bun_arch}/bun" /usr/local/bin/bun

    rm -rf "${bun_tmp}"
}

step "Checking for existing Bun installation"

if command -v bun > /dev/null 2>&1; then
    step "Bun already installed ($(bun --version)); skipping"
    STEP_BUN="already installed"
else
    install_bun
    step "Bun installed ($(bun --version))"
    STEP_BUN="installed"
fi

# ─── stop existing services (upgrade flow) ────────────────────────────────────

# Stop all services before replacing any files so binaries are never swapped
# out from under a running process.
SERVICES="dirigent-api dirigent-orchestrator dirigent-proxy dirigent-dashboard"

step "Stopping any running Dirigent services"

for svc in ${SERVICES}; do
    if systemctl is-active --quiet "${svc}" 2>/dev/null; then
        step "Stopping ${svc}"
        systemctl stop "${svc}"
    fi
done

# ─── download Go binaries ─────────────────────────────────────────────────────

download_binary() {
    local artifact="$1"
    local dest="$2"
    step "Downloading ${artifact}"
    curl -fsSL "${RELEASE_BASE}/${artifact}" -o "${dest}"
    chmod 755 "${dest}"
}

download_binary "dirigent-linux-${ARCH}"              /usr/local/bin/dirigent-api
download_binary "dirigent-orchestrator-linux-${ARCH}" /usr/local/bin/dirigent-orchestrator
download_binary "dirigent-proxy-linux-${ARCH}"        /usr/local/bin/dirigent-proxy

# ─── download and install dashboard ──────────────────────────────────────────

DASHBOARD_DIR="/opt/dirigent/dashboard"

step "Installing dashboard to ${DASHBOARD_DIR}"
mkdir -p "${DASHBOARD_DIR}"
curl -fsSL "${RELEASE_BASE}/dashboard.tar.gz" | tar -xz -C "${DASHBOARD_DIR}"

step "Installing dashboard production dependencies"
(cd "${DASHBOARD_DIR}" && bun install --production)

# ─── data directory ───────────────────────────────────────────────────────────

DATA_DIR="/var/lib/dirigent"

if [ ! -d "${DATA_DIR}" ]; then
    step "Creating data directory ${DATA_DIR}"
    mkdir -p "${DATA_DIR}"
else
    step "Data directory ${DATA_DIR} already exists; skipping"
fi

# ─── Docker network ───────────────────────────────────────────────────────────

DIRIGENT_NETWORK="dirigent"

step "Checking for Dirigent Docker network"

if docker network inspect "${DIRIGENT_NETWORK}" > /dev/null 2>&1; then
    step "Docker network '${DIRIGENT_NETWORK}' already exists; skipping"
    STEP_NETWORK="already exists"
else
    step "Creating Docker bridge network '${DIRIGENT_NETWORK}'"
    docker network create --driver bridge "${DIRIGENT_NETWORK}"
    step "Docker network '${DIRIGENT_NETWORK}' created"
    STEP_NETWORK="created"
fi

# ─── systemd units ────────────────────────────────────────────────────────────

step "Writing systemd unit files"

cat > /etc/systemd/system/dirigent-api.service << EOF
[Unit]
Description=Dirigent API
Documentation=https://github.com/ercadev/dirigent
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/dirigent-api
Environment=DIRIGENT_DATA=${DATA_DIR}/deployments.json
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/dirigent-orchestrator.service << EOF
[Unit]
Description=Dirigent orchestrator
Documentation=https://github.com/ercadev/dirigent
After=network.target docker.service dirigent-api.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/dirigent-orchestrator
Environment=DIRIGENT_DATA=${DATA_DIR}/deployments.json
Environment=DIRIGENT_API_URL=http://localhost:8080
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/dirigent-proxy.service << EOF
[Unit]
Description=Dirigent reverse proxy
Documentation=https://github.com/ercadev/dirigent
After=network.target docker.service dirigent-api.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/dirigent-proxy
Environment=DIRIGENT_DATA=${DATA_DIR}/deployments.json
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/dirigent-dashboard.service << EOF
[Unit]
Description=Dirigent dashboard
Documentation=https://github.com/ercadev/dirigent
After=network.target dirigent-api.service

[Service]
Type=simple
ExecStart=/usr/local/bin/bun ${DASHBOARD_DIR}/server.ts
Environment=PORT=3000
Environment=DIRIGENT_API_URL=http://localhost:8080
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# ─── enable and start services ────────────────────────────────────────────────

step "Reloading systemd daemon"
systemctl daemon-reload

for svc in ${SERVICES}; do
    step "Enabling and starting ${svc}"
    systemctl enable "${svc}"
    systemctl start "${svc}"
done

# ─── verify services ──────────────────────────────────────────────────────────

step "Verifying all services are active"

for svc in ${SERVICES}; do
    if ! systemctl is-active --quiet "${svc}"; then
        error "${svc} failed to start. Check logs with: journalctl -u ${svc} -n 50"
    fi
done

# ─── completion ───────────────────────────────────────────────────────────────

SERVER_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
if [ -z "${SERVER_IP}" ]; then
    SERVER_IP="<server-ip>"
fi

echo ""
echo "  ┌─────────────────────────────────────────────────────────────────────────┐"
echo "  │  Dirigent is ready                                                      │"
echo "  └─────────────────────────────────────────────────────────────────────────┘"
echo ""
echo "  Services:"
printf "    %-30s %s\n" "dirigent-api          :8080"  "$(systemctl is-active dirigent-api)"
printf "    %-30s %s\n" "dirigent-orchestrator  —"     "$(systemctl is-active dirigent-orchestrator)"
printf "    %-30s %s\n" "dirigent-proxy        :80"    "$(systemctl is-active dirigent-proxy)"
printf "    %-30s %s\n" "dirigent-dashboard    :3000"  "$(systemctl is-active dirigent-dashboard)"
echo ""
echo "  Dashboard:  http://${SERVER_IP}:3000"
echo "  API:        http://${SERVER_IP}:8080"
echo "  Proxy:      http://${SERVER_IP}:80"
echo ""
echo "  Note: The dashboard runs directly on :3000 rather than through the :80"
echo "  reverse proxy. This keeps it accessible even when no deployments are"
echo "  configured — the proxy only routes traffic to your deployed containers."
echo ""
echo "  Setup summary:"
echo "    Docker        ${STEP_DOCKER}"
echo "    Bun           ${STEP_BUN}"
echo "    Network       ${STEP_NETWORK}"
echo "    Data dir      ${DATA_DIR}"
echo "    Version       ${DIRIGENT_VERSION}"
echo ""
