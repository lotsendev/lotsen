#!/usr/bin/env bash
#
# Lotsen bootstrap installer
#
# Usage:
#   curl -fsSL https://github.com/lotsendev/lotsen/releases/latest/download/install.sh | sudo bash
#
# This script only installs the Lotsen CLI binary. Run `lotsen setup`
# afterwards to perform the full host setup.

set -euo pipefail

step() {
    echo "  -->  $*"
}

error() {
    echo "error: $*" >&2
    exit 1
}

step "Checking for root privileges"
if [ "$(id -u)" -ne 0 ]; then
    error "This script must be run as root or with sudo."
fi

step "Detecting operating system"
if [ ! -f /etc/os-release ]; then
    error "Cannot determine OS: /etc/os-release not found. Lotsen supports Ubuntu 22.04+ and Debian 11+."
fi

# shellcheck source=/dev/null
. /etc/os-release

OS_ID="${ID:-unknown}"
OS_VERSION_ID="${VERSION_ID:-0}"
OS_MAJOR="${OS_VERSION_ID%%.*}"

case "${OS_ID}" in
    ubuntu)
        if [ "${OS_MAJOR}" -lt 22 ]; then
            error "Ubuntu ${OS_VERSION_ID} is not supported. Minimum required version: Ubuntu 22.04."
        fi
        ;;
    debian)
        if [ "${OS_MAJOR}" -lt 11 ]; then
            error "Debian ${OS_VERSION_ID} is not supported. Minimum required version: Debian 11."
        fi
        ;;
    *)
        error "Unsupported operating system: ${OS_ID}. Lotsen supports Ubuntu 22.04+ and Debian 11+."
        ;;
esac

ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *) error "Unsupported architecture: ${ARCH}. Supported: x86_64, aarch64." ;;
esac

LOTSEN_VERSION="${LOTSEN_VERSION:-latest}"

if [ "${LOTSEN_VERSION}" = "latest" ]; then
    RELEASE_BASE="https://github.com/lotsendev/lotsen/releases/latest/download"
else
    RELEASE_BASE="https://github.com/lotsendev/lotsen/releases/download/${LOTSEN_VERSION}"
fi

step "Installing Lotsen CLI (${LOTSEN_VERSION}, linux/${ARCH})"
curl -fsSL "${RELEASE_BASE}/lotsen-cli-linux-${ARCH}" -o /usr/local/bin/lotsen
chmod 0755 /usr/local/bin/lotsen

echo ""
echo "  Lotsen CLI installed successfully."
echo ""
echo "  Next step:"
echo "    sudo lotsen setup"
echo ""
echo "  Tip: Run 'sudo lotsen setup' any time to update dashboard domain/auth."
echo ""
