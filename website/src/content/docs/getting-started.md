## Getting Started

Dirigent installs as three systemd services on your VPS. This guide walks you from a fresh Ubuntu or Debian server to running your first container in under five minutes.

## Prerequisites

- **OS:** Ubuntu 22.04 (Jammy) or later, or Debian 11 (Bullseye) or later.
- **Architecture:** x86_64 or aarch64.
- **Root access:** the installer must run as root or with `sudo`.
- **Open ports:** `80` (reverse proxy) and `8080` (API).
- **Docker:** if not already installed, the installer will install it for you.

## Installation

Run the following command on your VPS:

```bash
{{INSTALL_COMMAND}}
```

The installer will:

- Install Docker if it is not already present.
- Download and install the three Dirigent binaries.
- Create a Docker bridge network named `dirigent`.
- Write and enable three systemd units: `dirigent-api`, `dirigent-orchestrator`, and `dirigent-proxy`.
- Prompt for optional dashboard domain + Basic Auth setup (works in normal SSH sessions, including piped install commands).

> **Tip:** To pin a specific version, prefix the command with `DIRIGENT_VERSION=v0.0.2` before the curl.

## Verify the installation

Once the installer completes, confirm all services are running:

```bash
systemctl status dirigent-api
systemctl status dirigent-orchestrator
systemctl status dirigent-proxy
```

Each service should report `active (running)`. If one has failed, inspect its logs:

```bash
journalctl -u dirigent-api -n 50
```

## Access the dashboard

By default, the dashboard is available on port `8080` (served by `dirigent-api`):

```text
http://<your-vps-ip>:8080
```

The orchestrator has no public inbound port.

### Expose dashboard publicly (HTTPS + Basic Auth)

You can configure or update dashboard proxy exposure any time:

```bash
sudo dirigent setup
```

The setup command writes values to `/etc/dirigent/dirigent.env`, restarts the proxy, and enables dashboard access at `https://dashboard.example.com` and protected by HTTP Basic Auth.

### Proxy hardening

The proxy ships with three hardening profiles:

- `standard` (default): blocks `.env`, `.git`, and similar file probes.
- `strict`: adds broader scanner-target blocks and tighter rate limits — recommended for internet-facing hosts.
- `off`: disables hardening checks.

Pass the flag during setup:

```bash
sudo dirigent setup --proxy-hardening-profile strict
```

### Strict mode (recommended for public VPS)

Use strict host hardening and strict proxy hardening together when your server is internet-facing:

```bash
sudo dirigent setup --profile strict --proxy-hardening-profile strict
```

This applies stricter firewall and SSH defaults on the host, plus stronger scanner/path protections at the proxy layer.

For the full checklist (SSH key prerequisites, DNS, verification, and troubleshooting), see [Strict Mode Setup](/docs/strict-mode-setup).

## Your first deployment

### 1. Open the Deployments page

In the sidebar, click **Deployments**. This lists all containers Dirigent is currently managing.

### 2. Click "Create deployment"

A dialog opens with fields for your container configuration.

### 3. Fill in the details

For a quick test, deploy a simple nginx container:

- **Name:** `nginx`
- **Image:** `nginx:latest`
- **Ports:** `80:80`

### 4. Save

Click **Create**. Dirigent stores the deployment, and the orchestrator pulls the image and starts the container.

### 5. Verify status

Back on the Deployments table, wait for status to become `healthy`.

## Next steps

- Learn all available deployment fields in [Deployment Configuration](/docs/deployment-configuration).

## Upgrading

Dirigent can be upgraded in two ways:

- **Dashboard:** Open **Settings → System** and click **Check for updates**. If a new version is available, click **Upgrade** to trigger an in-place upgrade without leaving the browser.
- **CLI:** Re-run the installer on your VPS:
  ```bash
  curl -fsSL https://github.com/ercadev/dirigent-releases/releases/latest/download/install.sh | sudo bash
  sudo dirigent setup
  ```

Both paths perform an in-place upgrade and restart the services.
