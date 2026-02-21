#!/usr/bin/env bash
#
# Dirigent installer
#
# Usage:
#   curl -fsSL https://get.dirigent.sh | sudo bash
#   -- or --
#   sudo bash install.sh
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
        # Extract the major version component for comparison (22 from 22.04).
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

# ─── Docker installation ───────────────────────────────────────────────────────

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
else
    install_docker
    step "Docker installed ($(docker --version | head -1))"
fi

# ─── completion ───────────────────────────────────────────────────────────────

echo ""
echo "  Dirigent pre-install complete."
echo "  OS:     ${PRETTY_NAME:-${OS_ID} ${OS_VERSION_ID}}"
echo "  Docker: $(docker --version | head -1)"
echo ""
