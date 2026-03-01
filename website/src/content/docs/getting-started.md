## Getting Started

Lotsen installs as three systemd services on your VPS. This guide walks you from a fresh Ubuntu or Debian server to running your first container in under five minutes.

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
- Download and install the three Lotsen binaries.
- Create a Docker bridge network named `lotsen`.
- Write and enable three systemd units: `lotsen-api`, `lotsen-orchestrator`, and `lotsen-proxy`.
- Prompt for initial dashboard `/login` credentials in interactive setup.
- Prompt for optional dashboard domain + Basic Auth setup (works in normal SSH sessions, including piped install commands).

> **Tip:** To pin a specific version, prefix the command with `LOTSEN_VERSION=v0.0.2` before the curl.

## Verify the installation

Once the installer completes, confirm all services are running:

```bash
systemctl status lotsen-api
systemctl status lotsen-orchestrator
systemctl status lotsen-proxy
```

Each service should report `active (running)`. If one has failed, inspect its logs:

```bash
journalctl -u lotsen-api -n 50
```

## Access the dashboard

By default, the dashboard is available on port `8080` (served by `lotsen-api`):

```text
http://<your-vps-ip>:8080
```

The orchestrator has no public inbound port.

### First dashboard login user

In interactive mode, `lotsen setup` prompts for the initial dashboard `/login` username and password.

- Leave the password blank to auto-generate a strong password.
- The generated password is printed once at the end of setup.

`LOTSEN_AUTH_USER` and `LOTSEN_AUTH_PASSWORD` are bootstrap-only values. They are used to create the first user when the user database is empty, and ignored on later starts once users exist.

### Expose dashboard publicly (HTTPS + Basic Auth)

You can configure or update dashboard proxy exposure any time:

```bash
sudo lotsen setup
```

The setup command writes values to `/etc/lotsen/lotsen.env`, restarts the proxy, and enables dashboard access at `https://dashboard.example.com` and protected by HTTP Basic Auth.

### Proxy hardening

The proxy ships with three hardening profiles:

- `standard` (default): blocks `.env`, `.git`, and similar file probes.
- `strict`: adds broader scanner-target blocks and tighter rate limits — recommended for internet-facing hosts.
- `off`: disables hardening checks.

Pass the flag during setup:

```bash
sudo lotsen setup --proxy-hardening-profile strict
```

### Strict mode (recommended for public VPS)

Use strict host hardening and strict proxy hardening together when your server is internet-facing:

```bash
sudo lotsen setup --profile strict --proxy-hardening-profile strict
```

This applies stricter firewall and SSH defaults on the host, plus stronger scanner/path protections at the proxy layer.

For the full checklist (SSH key prerequisites, DNS, verification, and troubleshooting), see [Strict Mode Setup](/docs/strict-mode-setup).

## Your first deployment

### 1. Open the Deployments page

In the sidebar, click **Deployments**. This lists all containers Lotsen is currently managing.

### 2. Click "Create deployment"

A dialog opens with fields for your container configuration.

### 3. Fill in the details

For a quick test, deploy a simple nginx container:

- **Name:** `nginx`
- **Image:** `nginx:latest`
- **Ports:** `80:80`

### 4. Save

Click **Create**. Lotsen stores the deployment, and the orchestrator pulls the image and starts the container.

### 5. Verify status

Back on the Deployments table, wait for status to become `healthy`.

## Next steps

- Learn all available deployment fields in [Deployment Configuration](/docs/deployment-configuration).
- Review production caveats and runbooks in [Production Readiness](/docs/production-readiness).

## Upgrading

Lotsen can be upgraded in two ways:

- **Dashboard:** Open **Settings → System** and click **Check for updates**. If a new version is available, click **Upgrade** to trigger an in-place upgrade without leaving the browser.
- **CLI:** Re-run the installer on your VPS:
  ```bash
  curl -fsSL https://github.com/ercadev/dirigent-releases/releases/latest/download/install.sh | sudo bash
  sudo lotsen setup
  ```

Both paths perform an in-place upgrade and restart the services.
