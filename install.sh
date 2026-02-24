#!/usr/bin/env bash
#
# Dirigent bootstrap installer
#
# Usage:
#   curl -fsSL https://github.com/ercadev/dirigent-releases/releases/latest/download/install.sh | sudo bash
#
# This script only installs the Dirigent CLI binary. Run `dirigent setup`
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
    error "Cannot determine OS: /etc/os-release not found. Dirigent supports Ubuntu 22.04+ and Debian 11+."
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
        error "Unsupported operating system: ${OS_ID}. Dirigent supports Ubuntu 22.04+ and Debian 11+."
        ;;
esac

ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *) error "Unsupported architecture: ${ARCH}. Supported: x86_64, aarch64." ;;
esac

DIRIGENT_VERSION="${DIRIGENT_VERSION:-latest}"

if [ "${DIRIGENT_VERSION}" = "latest" ]; then
    RELEASE_BASE="https://github.com/ercadev/dirigent-releases/releases/latest/download"
else
    RELEASE_BASE="https://github.com/ercadev/dirigent-releases/releases/download/${DIRIGENT_VERSION}"
fi

step "Installing Dirigent CLI (${DIRIGENT_VERSION}, linux/${ARCH})"
curl -fsSL "${RELEASE_BASE}/dirigent-cli-linux-${ARCH}" -o /usr/local/bin/dirigent
chmod 0755 /usr/local/bin/dirigent

echo ""
echo "  Dirigent CLI installed successfully."
echo ""
echo "  Next step:"
echo "    sudo dirigent setup"
echo ""
