## Production Readiness

This guide documents what Lotsen guarantees today, what is partial, and what is not yet built, so you can make a production trial decision quickly.

## Reliability and failure behavior

Lotsen runs as three systemd services with `Restart=on-failure`:

- `lotsen-api`
- `lotsen-orchestrator`
- `lotsen-proxy`

### Service crash and restart

- If one Lotsen service exits unexpectedly, systemd restarts it automatically.
- Restart backoff is `5` seconds.
- Deployment state is persisted in `/var/lib/lotsen/deployments.json`, so desired state survives process restarts.

### Host reboot behavior

- All three services are enabled with systemd and start on boot.
- On startup, the orchestrator reconnects to Docker and resumes reconciliation.

### Reconciliation guarantees

- The orchestrator reconciles desired state to actual Docker state every `15s` by default.
- Proxy route table updates poll the shared state every `5s` by default.
- If Docker is temporarily unavailable, heartbeat and retry logic continue; recovery happens automatically when Docker returns.

## Backup and restore runbook

Lotsen has two persistence layers:

- control-plane state file: `/var/lib/lotsen/deployments.json`
- app data in bind-mounted host paths you define in deployment volumes

### Backup

1. Create a backup directory:

```bash
sudo mkdir -p /var/backups/lotsen
```

2. Backup the deployment state file:

```bash
sudo cp /var/lib/lotsen/deployments.json /var/backups/lotsen/deployments.json.$(date +%F-%H%M%S)
```

3. Backup each persistent app data path (example):

```bash
sudo tar -czf /var/backups/lotsen/postgres-data.$(date +%F-%H%M%S).tar.gz /srv/postgres-data
```

4. Optional integrity check:

```bash
sudo ls -lh /var/backups/lotsen
sudo tar -tzf /var/backups/lotsen/postgres-data.<timestamp>.tar.gz > /dev/null
```

### Restore

1. Stop Lotsen services:

```bash
sudo systemctl stop lotsen-api lotsen-orchestrator lotsen-proxy
```

2. Restore `deployments.json`:

```bash
sudo cp /var/backups/lotsen/deployments.json.<timestamp> /var/lib/lotsen/deployments.json
```

3. Restore each app volume archive to its original host path.

4. Start services:

```bash
sudo systemctl start lotsen-api lotsen-orchestrator lotsen-proxy
```

5. Verify recovery:

```bash
sudo systemctl status 'lotsen-*'
journalctl -u lotsen-orchestrator -n 100
```

## Security model and hardening checklist

### Security model

- Lotsen is single-host by design and runs as root-managed systemd services.
- API/dashboard default exposure is `:8080` unless dashboard domain exposure is configured.
- Proxy enforces domain routing and optional Basic Auth for dashboard access.
- Proxy hardening profiles (`standard` or `strict`) block common scanner and sensitive-path traffic.
- WAF rules are opt-in per deployment via `custom_rules`; no default deployment WAF rule set is applied.

### Internet-facing hardening checklist

1. Use `--profile strict --proxy-hardening-profile strict` for setup.
2. Disable SSH password auth and root login (strict mode does this automatically).
3. Keep only required inbound ports open (`80/443`, plus `22` for SSH management).
4. Set `LOTSEN_DASHBOARD_DOMAIN` and dashboard Basic Auth credentials.
5. Keep OS packages and Lotsen version updated.
6. Restrict who can read backup artifacts containing app data.

For full host hardening details, see [Strict Mode Setup](/docs/strict-mode-setup).

## HTTPS and TLS capability boundaries

### What works today

- Dashboard domain exposure supports automatic TLS via ACME when `LOTSEN_DASHBOARD_DOMAIN` is configured.
- HTTP requests are redirected to HTTPS for configured hosts.

### What does not work today

- App-domain TLS termination for arbitrary deployments is not generally available as a documented, supported workflow.
- For production app HTTPS today, place an upstream edge (for example Cloudflare, Caddy, or Nginx) in front of Lotsen, or terminate TLS in your app container.

## Feature claims matrix

| Capability | Status | Notes |
|---|---|---|
| Web dashboard | Available | Deploy, edit, inspect state and health. |
| Zero-downtime deploys | Available | Rolling replacement behavior in orchestrator. |
| Automatic container reconciliation | Available | Reconcile loop default every `15s`. |
| Integrated reverse proxy (HTTP routing) | Available | Domain-based routing on the built-in proxy. |
| Dashboard HTTPS + Basic Auth | Available | Domain exposure via setup flow. |
| App-domain HTTPS/TLS | Partial | Requires external edge or app-managed TLS. |
| GitOps workflow | Partial | Existing support, but not yet full enterprise controls. |
| RBAC / SSO / audit log | Planned | Not available in current release. |
| Multi-node / HA control plane | Planned | Single-host architecture currently. |

## Migration guides

### From Docker Compose

1. Pick one Compose service to migrate first.
2. Copy image, env vars, ports, and volume paths into a Lotsen deployment.
3. Keep host bind mount paths unchanged where possible.
4. Deploy in Lotsen and verify health/logs.
5. Cut traffic to the new endpoint.
6. Repeat service by service.

### From Portainer

1. Export or inspect existing container settings in Portainer.
2. Recreate each workload in Lotsen using the same image/tag and mounts.
3. Recreate published ports or use domain routing via Lotsen proxy.
4. Validate startup order and app connectivity.
5. Decommission Portainer-managed copies after verification.

## Production fit and non-fit

| Scenario | Fit | Why |
|---|---|---|
| Solo developer or small team on one VPS | Good fit | Fast setup and simple operations model. |
| Team needing basic dashboard + rolling deploys | Good fit | Good operational leverage without Kubernetes complexity. |
| Strict compliance environments needing RBAC/SSO/audit | Not fit today | These controls are not available yet. |
| Multi-region HA orchestration | Not fit today | Lotsen is single-host today. |

## Upgrade and compatibility policy

- Upgrades are in-place via `lotsen upgrade` or rerunning setup/install flow.
- Back up `/var/lib/lotsen/deployments.json` and persistent volumes before upgrading.
- Validate upgrade on a staging VPS before production.
- Pin to a specific release when you need deterministic rollout.
- Avoid skipping many releases at once; step through versions for lower risk.

## Troubleshooting playbook

### Dashboard unreachable

```bash
sudo systemctl status lotsen-api
sudo journalctl -u lotsen-api -n 100
```

### Deployments stuck in failed or deploying

```bash
sudo systemctl status lotsen-orchestrator
sudo journalctl -u lotsen-orchestrator -n 150
docker ps -a
```

### Domain not routing correctly

```bash
sudo systemctl status lotsen-proxy
sudo journalctl -u lotsen-proxy -n 150
```

### TLS certificate not issuing

1. Confirm DNS `A` record points to your VPS.
2. Confirm port `80` is reachable.
3. Check proxy logs for ACME errors.

## Performance envelope and day-2 checklist

### Current envelope

- Single VPS, single control plane, single local state file.
- No published formal benchmark suite yet for max deployment counts or request throughput.
- Conservative trial approach: start with non-critical workloads, observe resource use, then scale gradually.

### Day-2 operations checklist

1. Daily: check `systemctl status 'lotsen-*'` and recent service logs.
2. Daily: review failed deployment events in dashboard.
3. Weekly: verify backups for state file and app data volumes.
4. Weekly: apply OS security updates and restart if required.
5. Per release: upgrade in staging first, then production.
6. Monthly: test one restore drill end to end.
