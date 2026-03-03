#!/usr/bin/env bash
#
# Lotsen setup script
#
# Usage:
#   sudo lotsen setup
#
# This script is downloaded and executed by the Lotsen CLI. It can also
# be run directly for local testing:
#   sudo bash setup.sh
#
# To pin a specific version:
#   sudo DIRIGENT_VERSION=v0.0.2 bash setup.sh
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

ENV_FILE="/etc/dirigent/dirigent.env"

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

validate_domain() {
    local domain="$1"
    if [ -z "${domain}" ]; then
        return 1
    fi
    if [[ ! "${domain}" =~ ^[A-Za-z0-9.-]+$ ]]; then
        return 1
    fi
    if [[ "${domain}" != *.* ]]; then
        return 1
    fi
    return 0
}

write_dashboard_env() {
    local dashboard_domain="$1"
    local auth_user="$2"
    local auth_password="$3"
    local jwt_secret="$4"
    local auth_cookie_domain="$5"
    local tmp

    install -m 700 -d /etc/dirigent
    tmp=$(mktemp)

    if [ -f "${ENV_FILE}" ]; then
        awk '!/^(DIRIGENT|LOTSEN)_(DASHBOARD_(DOMAIN|USER|PASSWORD)|AUTH_(USER|PASSWORD|COOKIE_DOMAIN)|JWT_SECRET)=/' "${ENV_FILE}" > "${tmp}"
    fi

    if [ -n "${dashboard_domain}" ]; then
        {
            echo "DIRIGENT_DASHBOARD_DOMAIN=${dashboard_domain}"
            echo "LOTSEN_DASHBOARD_DOMAIN=${dashboard_domain}"
        } >> "${tmp}"
    fi

    {
        echo "DIRIGENT_JWT_SECRET=${jwt_secret}"
        echo "DIRIGENT_AUTH_USER=${auth_user}"
        echo "DIRIGENT_AUTH_PASSWORD=${auth_password}"
        echo "LOTSEN_JWT_SECRET=${jwt_secret}"
        echo "LOTSEN_AUTH_USER=${auth_user}"
        echo "LOTSEN_AUTH_PASSWORD=${auth_password}"
    } >> "${tmp}"

    if [ -n "${auth_cookie_domain}" ]; then
        {
            echo "DIRIGENT_AUTH_COOKIE_DOMAIN=${auth_cookie_domain}"
            echo "LOTSEN_AUTH_COOKIE_DOMAIN=${auth_cookie_domain}"
        } >> "${tmp}"
    fi

    install -m 600 "${tmp}" "${ENV_FILE}"
    rm -f "${tmp}"
}

read_env_value() {
    local key="$1"
    if [ ! -f "${ENV_FILE}" ]; then
        return 0
    fi
    grep "^${key}=" "${ENV_FILE}" | tail -n1 | cut -d'=' -f2- || true
}

generate_hex_secret() {
    local bytes="$1"
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -hex "${bytes}"
        return 0
    fi
    od -An -N "${bytes}" -tx1 /dev/urandom | tr -d ' \n'
}

choose_security_profile() {
    local selected="${DIRIGENT_SECURITY_PROFILE:-}"

    if [ -n "${selected}" ]; then
        echo "${selected}"
        return 0
    fi

    if [ "${DIRIGENT_UPGRADE:-0}" = "1" ]; then
        echo "standard"
        return 0
    fi

    if [ "${DIRIGENT_NON_INTERACTIVE:-0}" = "1" ] || [ ! -t 0 ]; then
        echo "standard"
        return 0
    fi

    echo "" >&2
    echo "Security profile" >&2
    echo "  1) strict (recommended)" >&2
    echo "  2) standard" >&2
    echo "  3) off" >&2
    read -r -p "Choose profile [1]: " profile_choice

    case "${profile_choice:-}" in
        ""|1|strict) echo "strict" ;;
        2|standard) echo "standard" ;;
        3|off) echo "off" ;;
        *)
            echo "strict"
            ;;
    esac
}

ensure_strict_prerequisites() {
    local users
    users=$(getent group sudo | awk -F: '{print $4}')
    if [ -z "${users}" ]; then
        error "Strict profile requires at least one non-root sudo user before SSH hardening can be applied."
    fi

    local has_key=0
    IFS=',' read -ra sudo_users <<< "${users}"
    for user in "${sudo_users[@]}"; do
        user=$(echo "${user}" | xargs)
        [ -z "${user}" ] && continue
        local home
        home=$(getent passwd "${user}" | cut -d: -f6)
        if [ -n "${home}" ] && [ -s "${home}/.ssh/authorized_keys" ]; then
            has_key=1
            break
        fi
    done

    if [ "${has_key}" -ne 1 ]; then
        error "Strict profile requires at least one sudo user with SSH keys in authorized_keys."
    fi
}

apply_strict_ssh_hardening() {
    step "Applying strict SSH hardening"

    if [ ! -f /etc/ssh/sshd_config ]; then
        error "Strict profile requested but /etc/ssh/sshd_config is missing"
    fi

    cp /etc/ssh/sshd_config /etc/ssh/sshd_config.dirigent.bak

    if grep -qE '^\s*PasswordAuthentication\s+' /etc/ssh/sshd_config; then
        sed -i 's/^\s*PasswordAuthentication\s\+.*/PasswordAuthentication no/' /etc/ssh/sshd_config
    else
        echo "PasswordAuthentication no" >> /etc/ssh/sshd_config
    fi

    if grep -qE '^\s*PermitRootLogin\s+' /etc/ssh/sshd_config; then
        sed -i 's/^\s*PermitRootLogin\s\+.*/PermitRootLogin no/' /etc/ssh/sshd_config
    else
        echo "PermitRootLogin no" >> /etc/ssh/sshd_config
    fi

    if systemctl list-unit-files | grep -q '^ssh\.service'; then
        systemctl reload ssh
    elif systemctl list-unit-files | grep -q '^sshd\.service'; then
        systemctl reload sshd
    fi
}

configure_firewall() {
    local profile="$1"

    if [ "${profile}" = "off" ]; then
        step "Security profile is off; skipping firewall configuration"
        return 0
    fi

    step "Configuring firewall"
    if ! command -v ufw > /dev/null 2>&1; then
        step "Installing ufw"
        apt-get install -y -q ufw
    fi

    local ssh_port
    ssh_port=$(awk '/^Port / {print $2; exit}' /etc/ssh/sshd_config 2>/dev/null || true)
    if [ -z "${ssh_port}" ]; then
        ssh_port="22"
    fi

    ufw --force reset
    ufw default deny incoming
    ufw default allow outgoing
    ufw allow "${ssh_port}/tcp"
    ufw allow 80/tcp
    ufw allow 443/tcp

    if [ "${DIRIGENT_OPEN_DASHBOARD_PORT:-0}" = "1" ]; then
        ufw allow 3000/tcp
    fi
    if [ "${DIRIGENT_OPEN_API_PORT:-0}" = "1" ]; then
        ufw allow 8080/tcp
    fi

    ufw --force enable
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

SECURITY_PROFILE="$(choose_security_profile)"
case "${SECURITY_PROFILE}" in
    strict|standard|off)
        ;;
    *)
        error "Invalid DIRIGENT_SECURITY_PROFILE='${SECURITY_PROFILE}'. Expected strict, standard, or off."
        ;;
esac

step "Using security profile: ${SECURITY_PROFILE}"
if [ "${SECURITY_PROFILE}" = "strict" ]; then
    ensure_strict_prerequisites
fi

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

# ─── stop existing services (upgrade flow) ────────────────────────────────────

# Stop all services before replacing any files so binaries are never swapped
# out from under a running process. Also stop and disable the legacy monolithic
# service from older installs so it does not hold port 8080.
SERVICES="lotsen-api lotsen-orchestrator lotsen-proxy"
LEGACY_SERVICES="dirigent-api dirigent-orchestrator dirigent-proxy"

step "Stopping any running Dirigent services"

if systemctl is-active --quiet dirigent 2>/dev/null; then
    step "Stopping legacy dirigent.service"
    systemctl stop dirigent
    systemctl disable dirigent 2>/dev/null || true
fi

if systemctl is-active --quiet dirigent-dashboard 2>/dev/null; then
    step "Stopping legacy dirigent-dashboard"
    systemctl stop dirigent-dashboard
fi
systemctl disable dirigent-dashboard 2>/dev/null || true

for svc in ${SERVICES} ${LEGACY_SERVICES}; do
    if systemctl is-active --quiet "${svc}" 2>/dev/null; then
        step "Stopping ${svc}"
        systemctl stop "${svc}"
    fi
done

for svc in ${LEGACY_SERVICES}; do
    if systemctl is-enabled --quiet "${svc}" 2>/dev/null; then
        step "Disabling legacy ${svc}"
        systemctl disable "${svc}" 2>/dev/null || true
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

download_binary "lotsen-api-linux-${ARCH}"              /usr/local/bin/lotsen-api
download_binary "lotsen-orchestrator-linux-${ARCH}"     /usr/local/bin/lotsen-orchestrator
download_binary "lotsen-proxy-linux-${ARCH}"            /usr/local/bin/lotsen-proxy

# ─── data directory ───────────────────────────────────────────────────────────

DATA_DIR="/var/lib/dirigent"

if [ ! -d "${DATA_DIR}" ]; then
    step "Creating data directory ${DATA_DIR}"
    mkdir -p "${DATA_DIR}"
else
    step "Data directory ${DATA_DIR} already exists; skipping"
fi

# ─── dashboard public exposure setup ──────────────────────────────────────────

DASHBOARD_DOMAIN="${DIRIGENT_DASHBOARD_DOMAIN:-}"
AUTH_USER="${DIRIGENT_AUTH_USER:-${LOTSEN_AUTH_USER:-}}"
AUTH_PASSWORD="${DIRIGENT_AUTH_PASSWORD:-${LOTSEN_AUTH_PASSWORD:-}}"
JWT_SECRET="${DIRIGENT_JWT_SECRET:-${LOTSEN_JWT_SECRET:-}}"
AUTH_COOKIE_DOMAIN="${DIRIGENT_AUTH_COOKIE_DOMAIN:-${LOTSEN_AUTH_COOKIE_DOMAIN:-}}"
GENERATED_AUTH_PASSWORD=0
GENERATED_JWT_SECRET=0
EXISTING_DASHBOARD_DOMAIN=""
EXISTING_AUTH_USER=""
EXISTING_AUTH_PASSWORD=""
EXISTING_JWT_SECRET=""
EXISTING_AUTH_COOKIE_DOMAIN=""

if [ -f "${ENV_FILE}" ]; then
    EXISTING_DASHBOARD_DOMAIN=$(read_env_value "DIRIGENT_DASHBOARD_DOMAIN")
    EXISTING_AUTH_USER=$(read_env_value "DIRIGENT_AUTH_USER")
    EXISTING_AUTH_PASSWORD=$(read_env_value "DIRIGENT_AUTH_PASSWORD")
    EXISTING_JWT_SECRET=$(read_env_value "DIRIGENT_JWT_SECRET")
    EXISTING_AUTH_COOKIE_DOMAIN=$(read_env_value "DIRIGENT_AUTH_COOKIE_DOMAIN")
    if [ -z "${EXISTING_DASHBOARD_DOMAIN}" ]; then
        EXISTING_DASHBOARD_DOMAIN=$(read_env_value "LOTSEN_DASHBOARD_DOMAIN")
    fi
    if [ -z "${EXISTING_AUTH_USER}" ]; then
        EXISTING_AUTH_USER=$(read_env_value "LOTSEN_AUTH_USER")
    fi
    if [ -z "${EXISTING_AUTH_PASSWORD}" ]; then
        EXISTING_AUTH_PASSWORD=$(read_env_value "LOTSEN_AUTH_PASSWORD")
    fi
    if [ -z "${EXISTING_JWT_SECRET}" ]; then
        EXISTING_JWT_SECRET=$(read_env_value "LOTSEN_JWT_SECRET")
    fi
    if [ -z "${EXISTING_AUTH_COOKIE_DOMAIN}" ]; then
        EXISTING_AUTH_COOKIE_DOMAIN=$(read_env_value "LOTSEN_AUTH_COOKIE_DOMAIN")
    fi

    if [ -z "${DASHBOARD_DOMAIN}" ] && [ -n "${EXISTING_DASHBOARD_DOMAIN}" ]; then
        DASHBOARD_DOMAIN="${EXISTING_DASHBOARD_DOMAIN}"
    fi
    if [ -z "${AUTH_USER}" ] && [ -n "${EXISTING_AUTH_USER}" ]; then
        AUTH_USER="${EXISTING_AUTH_USER}"
    fi
    if [ -z "${AUTH_PASSWORD}" ] && [ -n "${EXISTING_AUTH_PASSWORD}" ]; then
        AUTH_PASSWORD="${EXISTING_AUTH_PASSWORD}"
    fi
    if [ -z "${JWT_SECRET}" ] && [ -n "${EXISTING_JWT_SECRET}" ]; then
        JWT_SECRET="${EXISTING_JWT_SECRET}"
    fi
    if [ -z "${AUTH_COOKIE_DOMAIN}" ] && [ -n "${EXISTING_AUTH_COOKIE_DOMAIN}" ]; then
        AUTH_COOKIE_DOMAIN="${EXISTING_AUTH_COOKIE_DOMAIN}"
    fi
fi

if [ -z "${AUTH_USER}" ]; then
    AUTH_USER="admin"
fi
if [ -z "${JWT_SECRET}" ]; then
    JWT_SECRET=$(generate_hex_secret 32)
    GENERATED_JWT_SECRET=1
fi

if [ -t 0 ] && [ "${DIRIGENT_NON_INTERACTIVE:-0}" != "1" ] && [ "${DIRIGENT_UPGRADE:-0}" != "1" ] && [ -z "${AUTH_PASSWORD}" ]; then
    echo ""
    echo "Dashboard /login bootstrap credentials"
    echo "  These credentials are used for the first dashboard login user."

    read -r -p "Dashboard login username [${AUTH_USER}]: " INPUT_AUTH_USER
    if [ -n "${INPUT_AUTH_USER}" ]; then
        AUTH_USER="${INPUT_AUTH_USER}"
    fi

    while true; do
        read -r -s -p "Dashboard login password (leave blank to auto-generate): " INPUT_AUTH_PASSWORD
        echo ""
        if [ -z "${INPUT_AUTH_PASSWORD}" ]; then
            AUTH_PASSWORD=$(generate_hex_secret 16)
            GENERATED_AUTH_PASSWORD=1
            break
        fi

        read -r -s -p "Confirm dashboard login password: " INPUT_AUTH_PASSWORD_CONFIRM
        echo ""
        if [ "${INPUT_AUTH_PASSWORD}" != "${INPUT_AUTH_PASSWORD_CONFIRM}" ]; then
            echo "Passwords do not match. Try again."
            continue
        fi

        AUTH_PASSWORD="${INPUT_AUTH_PASSWORD}"
        break
    done
fi

if [ -z "${AUTH_PASSWORD}" ]; then
    AUTH_PASSWORD=$(generate_hex_secret 16)
    GENERATED_AUTH_PASSWORD=1
fi

if [ -t 0 ] && [ "${DIRIGENT_NON_INTERACTIVE:-0}" != "1" ] && [ "${DIRIGENT_UPGRADE:-0}" != "1" ]; then
    echo ""
    echo "Dashboard public exposure setup"
    echo "  Configure HTTPS on a dedicated domain (optional)."
    read -r -p "Expose dashboard publicly through the proxy? [y/N]: " EXPOSE_DASHBOARD
    if [[ "${EXPOSE_DASHBOARD}" =~ ^[Yy]$ ]]; then
        while true; do
            if [ -n "${DASHBOARD_DOMAIN}" ]; then
                read -r -p "Dashboard domain [${DASHBOARD_DOMAIN}]: " INPUT_DASHBOARD_DOMAIN
                if [ -n "${INPUT_DASHBOARD_DOMAIN}" ]; then
                    DASHBOARD_DOMAIN="${INPUT_DASHBOARD_DOMAIN}"
                fi
            else
                read -r -p "Dashboard domain (e.g. dashboard.example.com): " DASHBOARD_DOMAIN
            fi

            if validate_domain "${DASHBOARD_DOMAIN}"; then
                break
            fi
            echo "Invalid domain. Use a valid hostname like dashboard.example.com"
        done

    else
        DASHBOARD_DOMAIN=""
    fi
fi

if [ -n "${DASHBOARD_DOMAIN}" ]; then
    if ! validate_domain "${DASHBOARD_DOMAIN}"; then
        error "DIRIGENT_DASHBOARD_DOMAIN is set but invalid. Example: dashboard.example.com"
    fi
fi

step "Writing shared environment file"
write_dashboard_env "${DASHBOARD_DOMAIN}" "${AUTH_USER}" "${AUTH_PASSWORD}" "${JWT_SECRET}" "${AUTH_COOKIE_DOMAIN}"

configure_firewall "${SECURITY_PROFILE}"
if [ "${SECURITY_PROFILE}" = "strict" ]; then
    apply_strict_ssh_hardening
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

cat > /etc/systemd/system/lotsen-api.service << EOF
[Unit]
Description=Lotsen API
Documentation=https://github.com/ercadev/dirigent
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/lotsen-api
EnvironmentFile=-${ENV_FILE}
Environment=DIRIGENT_DATA=${DATA_DIR}/deployments.json
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/lotsen-orchestrator.service << EOF
[Unit]
Description=Lotsen orchestrator
Documentation=https://github.com/ercadev/dirigent
After=network.target docker.service lotsen-api.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/lotsen-orchestrator
EnvironmentFile=-${ENV_FILE}
Environment=DIRIGENT_DATA=${DATA_DIR}/deployments.json
Environment=DIRIGENT_API_URL=http://localhost:8080
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/lotsen-proxy.service << EOF
[Unit]
Description=Lotsen reverse proxy
Documentation=https://github.com/ercadev/dirigent
After=network.target docker.service lotsen-api.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/lotsen-proxy
EnvironmentFile=-${ENV_FILE}
Environment=DIRIGENT_DATA=${DATA_DIR}/deployments.json
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

rm -f /etc/systemd/system/dirigent-api.service
rm -f /etc/systemd/system/dirigent-orchestrator.service
rm -f /etc/systemd/system/dirigent-proxy.service

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
printf "    %-30s %s\n" "lotsen-api            :8080"  "$(systemctl is-active lotsen-api)"
printf "    %-30s %s\n" "lotsen-orchestrator    —"     "$(systemctl is-active lotsen-orchestrator)"
printf "    %-30s %s\n" "lotsen-proxy          :80"    "$(systemctl is-active lotsen-proxy)"
echo ""
if [ -n "${DASHBOARD_DOMAIN}" ]; then
    echo "  Dashboard:  https://${DASHBOARD_DOMAIN}"
else
    echo "  Dashboard:  http://${SERVER_IP}:8080"
fi
echo "  Dashboard login user: ${AUTH_USER}"
if [ "${GENERATED_AUTH_PASSWORD}" = "1" ]; then
    echo "  Dashboard login password: ${AUTH_PASSWORD}"
fi
if [ "${GENERATED_JWT_SECRET}" = "1" ]; then
    echo "  Dashboard auth secret was generated automatically."
fi
echo "  API:        http://${SERVER_IP}:8080"
echo "  Proxy:      http://${SERVER_IP}:80"
echo ""
if [ -n "${DASHBOARD_DOMAIN}" ]; then
    echo "  Note: Ensure DNS A record for ${DASHBOARD_DOMAIN} points to this server"
    echo "  and port 80 is open so certificates can be issued."
else
    echo "  Note: The dashboard is served directly by lotsen-api on :8080."
    echo "  Configure DIRIGENT_DASHBOARD_DOMAIN in setup to expose it through"
    echo "  the :80/:443 reverse proxy with TLS."
fi
echo ""
echo "  Setup summary:"
echo "    Docker        ${STEP_DOCKER}"
echo "    Network       ${STEP_NETWORK}"
echo "    Data dir      ${DATA_DIR}"
echo "    Version       ${DIRIGENT_VERSION}"
echo ""
