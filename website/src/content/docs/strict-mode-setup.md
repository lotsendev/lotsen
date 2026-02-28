## Strict Mode Setup

Use this guide when you want the strongest default hardening for an internet-facing VPS.

## What strict mode means

Dirigent has two separate security controls:

- **Host security profile** (`--profile`): configures host-level firewall and SSH behavior.
- **Proxy hardening profile** (`--proxy-hardening-profile`): filters suspicious inbound HTTP paths and scanner traffic.

For production, use `strict` for both.

## Before you enable strict mode

Strict host hardening changes SSH behavior. Complete these checks first:

1. Create at least one non-root sudo user.
2. Confirm that user has SSH keys in `~/.ssh/authorized_keys`.
3. Verify you can log in with that key before applying strict mode.
4. Ensure ports `80` and `443` are reachable from the internet.

If you expose the dashboard on a domain, also prepare DNS:

1. Set a DNS `A` record (for example `dashboard.example.com`) to your VPS IP.
2. Keep port `80` open so ACME HTTP-01 challenges can complete.

## Install with strict mode

If Dirigent is not installed yet:

```bash
curl -fsSL https://github.com/ercadev/dirigent-releases/releases/latest/download/install.sh | sudo bash
sudo dirigent setup --profile strict --proxy-hardening-profile strict
```

If Dirigent is already installed, re-run setup:

```bash
sudo dirigent setup --profile strict --proxy-hardening-profile strict
```

For automation/non-interactive environments:

```bash
sudo DIRIGENT_SECURITY_PROFILE=strict DIRIGENT_PROXY_HARDENING_PROFILE=strict DIRIGENT_NON_INTERACTIVE=1 dirigent setup
```

## Optional: expose dashboard with HTTPS and Basic Auth

Interactive mode:

```bash
sudo dirigent setup --profile strict --proxy-hardening-profile strict
```

Then choose dashboard exposure and provide:

- dashboard domain,
- basic auth username,
- basic auth password.

Non-interactive mode:

```bash
sudo DIRIGENT_SECURITY_PROFILE=strict DIRIGENT_PROXY_HARDENING_PROFILE=strict DIRIGENT_DASHBOARD_DOMAIN=dashboard.example.com DIRIGENT_DASHBOARD_USER=admin DIRIGENT_DASHBOARD_PASSWORD='change-me' DIRIGENT_NON_INTERACTIVE=1 dirigent setup
```

## What gets hardened

### Host profile: `strict`

- Configures UFW with default deny inbound and allows only SSH + `80/443` by default.
- Disables SSH password authentication.
- Disables SSH root login.

### Proxy profile: `strict`

- Blocks sensitive file and dot-path probes (for example `/.env`, `/.git`, `/.ssh`).
- Blocks common scanner and exploit probe paths (for example `/wp-admin`, `/swagger*`, `/actuator`, `/phpmyadmin`).
- Applies tighter suspicious-request thresholds and longer temporary IP blocks.

## Verify strict mode is active

Run these checks on the VPS:

```bash
systemctl status dirigent-api
systemctl status dirigent-orchestrator
systemctl status dirigent-proxy
sudo ufw status verbose
```

Inspect proxy logs if needed:

```bash
journalctl -u dirigent-proxy -n 100
```

If dashboard exposure is enabled, confirm HTTPS works:

```bash
curl -I https://dashboard.example.com
```

## Troubleshooting

If certificates are not issued:

- confirm DNS points to the VPS,
- confirm port `80` is open and reachable,
- check `dirigent-proxy` logs for ACME errors.

If strict mode is too restrictive for your environment, re-run setup with a less strict profile:

```bash
sudo dirigent setup --profile standard --proxy-hardening-profile standard
```
