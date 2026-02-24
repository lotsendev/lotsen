## Getting Started

Dirigent installs as four systemd services on your VPS. This guide walks you from a fresh Ubuntu or Debian server to running your first container in under five minutes.

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
- Install Bun (required to run the dashboard server).
- Download and install the four Dirigent binaries.
- Create a Docker bridge network named `dirigent`.
- Write and enable four systemd units: `dirigent-api`, `dirigent-orchestrator`, `dirigent-proxy`, and `dirigent-dashboard`.
- Prompt for optional dashboard domain + Basic Auth setup (works in normal SSH sessions, including piped install commands).

> **Tip:** To pin a specific version, prefix the command with `DIRIGENT_VERSION=v0.0.2` before the curl.

## Verify the installation

Once the installer completes, confirm all four services are running:

```bash
systemctl status dirigent-api
systemctl status dirigent-orchestrator
systemctl status dirigent-proxy
systemctl status dirigent-dashboard
```

Each service should report `active (running)`. If one has failed, inspect its logs:

```bash
journalctl -u dirigent-api -n 50
```

## Access the dashboard

By default, the dashboard is available directly on port `3000`:

```text
http://<your-vps-ip>:3000
```

The dashboard connects to the local API on port 8080. The orchestrator has no public inbound port.

### Expose dashboard publicly (HTTPS + Basic Auth)

You can configure or update dashboard proxy exposure any time:

```bash
sudo dirigent setup
```

The setup command writes values to `/etc/dirigent/dirigent.env`, restarts the proxy, and enables dashboard access at `https://dashboard.example.com` and protected by HTTP Basic Auth.

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
- Re-run installer to upgrade Dirigent safely.
