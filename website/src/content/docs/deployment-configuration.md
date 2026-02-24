## Deployment Configuration

A deployment is the central object in Dirigent. It describes a container you want to keep running — the image to use, how to expose it, and what environment it needs. The orchestrator continuously reconciles your running containers against these declarations.

## Field reference

| Field   | Type     | Required | Description |
|---------|----------|----------|-------------|
| name    | string   | Yes      | A human-readable identifier for the deployment. Used as the container name in Docker. Must be unique across all deployments. |
| image   | string   | Yes      | The Docker image to run, including tag. The orchestrator pulls this image before starting the container. Example: `nginx:1.27` or `ghcr.io/myorg/api:latest`. |
| ports   | string[] | No       | Port mappings in `host:container` format. Each entry maps a port on the VPS host to a port inside the container. Example: `["80:80", "443:443"]`. |
| volumes | string[] | No       | Volume mounts in `host-path:container-path` format. The host path must be an absolute path on the VPS. Example: `["/data/postgres:/var/lib/postgresql/data"]`. |
| envs    | object   | No       | Environment variables passed into the container as a key-value map. Values are stored in the Dirigent data file on disk. Example: `{"DATABASE_URL": "postgres://..."}`. |
| domain  | string   | No       | A fully-qualified domain name to route to this container via the integrated reverse proxy. Point your DNS A record to the VPS IP, and Dirigent will forward HTTP traffic on port 80. Example: `api.example.com`. |

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

Volumes persist data across container restarts and re-deployments:

```text
"/var/lib/dirigent/myapp:/data"  // host path : container path
```

> **Note:** The host path must exist before the deployment is created. Dirigent does not create directories on your behalf. Use `mkdir -p /path/to/dir` on the VPS first.

## Environment variables

Environment variables are entered as key-value pairs in the dashboard. They are stored in the Dirigent state file at `/var/lib/dirigent/deployments.json` — a file on disk that is only readable by root.

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
3. Ensure the container exposes a port on its internal network (no host port mapping required — the proxy communicates over the `dirigent` Docker network).

> **Note:** The proxy currently handles HTTP only. TLS termination is on the roadmap for a future release.

## Deployment lifecycle

A deployment moves through the following states:

| Status    | Meaning |
|-----------|---------|
| idle      | The deployment has been created but the orchestrator has not yet acted on it. |
| deploying | The orchestrator is pulling the image and starting the container. |
| healthy   | The container is running and passing health checks. |
| failed    | The container exited unexpectedly or the image pull failed. Check the logs panel on the deployment detail page for the error. |
