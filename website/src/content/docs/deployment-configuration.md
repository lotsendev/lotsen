## Deployment Configuration

A deployment is the central object in Lotsen. It describes a container you want to keep running — the image to use, how to expose it, and what environment it needs. The orchestrator continuously reconciles your running containers against these declarations.

## Field reference

| Field   | Type     | Required | Description |
|---------|----------|----------|-------------|
| name    | string   | Yes      | A human-readable identifier for the deployment. Used as the container name in Docker. Must be unique across all deployments. |
| image   | string   | Yes      | The Docker image to run, including tag. The orchestrator pulls this image before starting the container. Example: `nginx:1.27` or `ghcr.io/myorg/api:latest`. |
| ports   | string[] | No       | Port mappings in `host:container` format. Each entry maps a port on the VPS host to a port inside the container. Example: `["80:80", "443:443"]`. |
| volume_mounts | object[] | No | Volume mounts with explicit mode: managed or bind. Managed mounts are created automatically under `/var/lib/lotsen/volumes/<deployment>/<volume>`. Managed mounts also support optional `uid`, `gid`, and `dir_mode` ownership settings. Example: `[{"mode":"managed","source":"postgres-data","target":"/var/lib/postgresql/data","uid":5050,"gid":5050,"dir_mode":"0770"}]`. |
| file_mounts | object[] | No | Managed text files created by Lotsen and mounted into the container. Useful for apps that need config files (for example Prometheus). Supports `source`, `target`, `content`, optional `uid`, `gid`, `file_mode`, and `read_only`. |
| volumes | string[] | No | Backward-compatible raw bind format (`host-path:container-path`). Prefer `volume_mounts` for new deployments. |
| envs    | object   | No       | Environment variables passed into the container as a key-value map. Values are stored in the Lotsen data file on disk. Example: `{"DATABASE_URL": "postgres://..."}`. |
| domain  | string   | No       | A fully-qualified domain name to route to this container via the integrated reverse proxy. Point your DNS A record to the VPS IP, and Lotsen will forward HTTP traffic on port 80. Example: `api.example.com`. |
| basic_auth | object | No       | Require HTTP Basic Auth on the proxy route for this deployment. Only applies when `domain` is set. Contains a `users` list of `{ username, password }` pairs. |
| security | object | No | Per-deployment traffic security settings. Includes `waf_enabled`, `waf_mode` (`detection` or `enforcement`), `ip_denylist`, `ip_allowlist`, and `custom_rules`. |

## Ports

Port mappings use the Docker format:

```text
host-port:container-port
```

Examples:

```text
"8080:80"   // expose container port 80 as host port 8080
"3306:3306" // MySQL
"5432:5432" // Postgres
```

Omitting ports means the container is not directly accessible from the host. Use the `domain` field with the reverse proxy instead for HTTP services.

## Volumes

Volumes persist data across container restarts and re-deployments.

### Managed volume mode (recommended)

Managed volumes let Lotsen create and own host directories for you:

```json
"volume_mounts": [
  {
    "mode": "managed",
    "source": "postgres-data",
    "target": "/var/lib/postgresql/data"
  }
]
```

The first deploy creates:

```text
/var/lib/lotsen/volumes/<deployment-id>/postgres-data
```

This path is remounted automatically on redeploy.

For non-root images (for example pgAdmin), managed mounts are created with writable permissions by default (`0777`) so first boot can succeed without manual host `chown`. You can also override ownership explicitly:

```json
"volume_mounts": [
  {
    "mode": "managed",
    "source": "pgadmin-data",
    "target": "/var/lib/pgadmin",
    "uid": 5050,
    "gid": 5050,
    "dir_mode": "0770"
  }
]
```

### Advanced bind mount mode

Use bind mounts when you need full host-path control:

```text
"volume_mounts": [
  {
    "mode": "bind",
    "source": "/srv/postgres-data",
    "target": "/var/lib/postgresql/data"
  }
]
```

### Postgres example (managed)

```json
{
  "name": "postgres",
  "image": "postgres:16",
  "ports": ["5432:5432"],
  "envs": {
    "POSTGRES_DB": "app",
    "POSTGRES_USER": "app",
    "POSTGRES_PASSWORD": "change-me"
  },
  "volume_mounts": [
    {
      "mode": "managed",
      "source": "data",
      "target": "/var/lib/postgresql/data"
    }
  ]
}
```

## Config files (`file_mounts`)

Some images need a real config file on disk. Use `file_mounts` to write file content from the dashboard and mount it into the container.

```json
"file_mounts": [
  {
    "source": "prometheus.yml",
    "target": "/etc/prometheus/prometheus.yml",
    "content": "global:\n  scrape_interval: 15s\n",
    "read_only": true
  }
]
```

Managed files are created under:

```text
/var/lib/lotsen/files/<deployment-id>/<source>
```

This makes it possible to run services like Prometheus without SSH access just to create config files.

## Environment variables

Environment variables are entered as key-value pairs in the dashboard. They are stored in the Lotsen state file at `/var/lib/lotsen/deployments.json` — a file on disk that is only readable by root.

> **Security note:** Do not store highly sensitive credentials (private keys, payment tokens) in environment variables if your VPS is not hardened. For production secrets management, consider mounting a secrets file as a volume instead.

Common usage patterns:

```text
DATABASE_URL=postgres://user:pass@localhost:5432/mydb
NODE_ENV=production
PORT=3000
```

## Domain and reverse proxy

The integrated reverse proxy listens on port 80 and routes incoming HTTP requests to the correct container based on the `Host` header.

To expose a deployment via a domain:

1. Set the **Domain** field to your fully-qualified domain name (e.g. `app.example.com`).
2. Create a DNS A record pointing that domain to your VPS IP address.
3. Ensure the container exposes a port on its internal network (no host port mapping required — the proxy communicates over the `lotsen` Docker network).

> **Note:** The proxy currently handles HTTP only. TLS termination is on the roadmap for a future release.

## Basic Auth

You can protect a domain-exposed deployment with HTTP Basic Auth. When configured, the proxy challenges any incoming request before forwarding it to the container.

Set credentials in the dashboard when creating or editing a deployment. Multiple users can be added — useful for granting access to teammates without sharing a single credential.

> **Security note:** Basic Auth over plain HTTP transmits credentials in base64. Combine with HTTPS termination (via an upstream proxy or CDN) for production use.

## WAF rules

WAF behavior is configured per deployment in **Security** on the deployment details page.

- `waf_enabled`: toggles WAF for this deployment only.
- `waf_mode`: `detection` (log only) or `enforcement` (block matching requests).
- `custom_rules`: one ModSecurity/Coraza rule per line.

By default, `custom_rules` is empty. Add only the rules you want for each deployment.

Coraza rule syntax reference:

- [Coraza SecLang directives](https://coraza.io/docs/seclang/directives/)

Example starter rules:

```text
# Block direct probes for common sensitive files.
SecRule REQUEST_URI "@rx (?i)^/(\.env|\.git|\.svn|\.DS_Store)$" "id:110001,phase:1,deny,status:403,log,msg:'Sensitive file probe'"

# Block obvious SQL injection patterns in query strings.
SecRule ARGS "@rx (?i)(union\s+select|or\s+1=1|information_schema)" "id:110002,phase:2,deny,status:403,log,msg:'SQLi pattern in args'"

# Block path traversal attempts.
SecRule REQUEST_URI "@rx (\.\./|%2e%2e%2f|%252e%252e%252f)" "id:110003,phase:1,deny,status:403,log,msg:'Path traversal attempt'"
```

> **Rule ID note:** Keep rule IDs unique per deployment to avoid conflicts.

### Copy-paste starter pack

You can start with these additional rules and then tune based on your app behavior.

```text
# Block command injection separators in common input args.
SecRule ARGS "@rx (?i)(;|\|\||&&|`|\$\()" "id:110004,phase:2,deny,status:403,log,msg:'Command injection pattern in args'"

# Block requests with known scanner user agents.
SecRule REQUEST_HEADERS:User-Agent "@rx (?i)(sqlmap|nikto|nmap|masscan|acunetix)" "id:110005,phase:1,deny,status:403,log,msg:'Scanner user-agent blocked'"

# Block direct access to common admin/debug endpoints.
SecRule REQUEST_URI "@rx (?i)^/(phpmyadmin|wp-admin|wp-login\.php|actuator|_debugbar)" "id:110006,phase:1,deny,status:403,log,msg:'Admin/debug endpoint probe'"

# Restrict methods to common web/API verbs.
SecRule REQUEST_METHOD "!@within GET POST PUT PATCH DELETE OPTIONS HEAD" "id:110007,phase:1,deny,status:405,log,msg:'Unexpected HTTP method'"

# Block oversized query strings (basic abuse guard).
SecRule QUERY_STRING "@gt 2048" "id:110008,phase:1,deny,status:414,log,msg:'Query string too long'"
```

Suggested rollout:

1. Enable `waf_mode: detection` first and review access logs.
2. Keep rules that catch bad traffic without false positives.
3. Switch to `waf_mode: enforcement` after validation.

## Deployment lifecycle

A deployment moves through the following states:

| Status    | Meaning |
|-----------|---------|
| idle      | The deployment has been created but the orchestrator has not yet acted on it. |
| deploying | The orchestrator is pulling the image and starting the container. |
| healthy   | The container is running and passing health checks. |
| failed    | The container exited unexpectedly or the image pull failed. Check the logs panel on the deployment detail page for the error. |
