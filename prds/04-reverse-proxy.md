# PRD: Integrated Reverse Proxy / Load Balancer

## Problem Statement

Exposing a Docker container to the internet on a VPS requires manually configuring a reverse proxy (nginx, Caddy, Traefik), setting up TLS certificates, and keeping that configuration in sync as containers are created, updated, or removed. This is complex, error-prone, and becomes a maintenance burden over time — especially when running multiple services on the same VPS.

## Solution

Lotsen includes an integrated reverse proxy that automatically handles domain-based routing and TLS certificate provisioning. When a deployment has a domain configured, it becomes publicly reachable over HTTPS with no additional setup. The proxy configuration is updated automatically as deployments are created, updated, or removed.

## User Stories

1. As a developer, I want to assign a domain to a deployment so that it becomes publicly reachable from the internet without configuring a proxy manually.
2. As a developer, I want TLS certificates to be provisioned automatically via Let's Encrypt when I assign a domain, so that my service is served over HTTPS without manual certificate management.
3. As a developer, I want TLS certificates to renew automatically before they expire, so that my services don't go down due to certificate expiry.
4. As a developer, I want HTTP traffic to be automatically redirected to HTTPS, so that my service is always served securely regardless of how users access it.
5. As a developer, I want traffic for different domains to be routed to their respective deployments, so that I can run multiple services on a single VPS.
6. As a developer, I want deployments without a domain to remain inaccessible from the internet, so that I can run internal services safely.
7. As a developer, I want the proxy to update automatically when I save a deployment with a new or changed domain, so that routing changes take effect immediately without restarting anything.
8. As a developer, I want the proxy to route traffic to the new container after a zero-downtime redeploy, so that domain routing stays correct after updates.
9. As a developer, I want the proxy to stop routing to a deployment when it is removed, so that deleted services are no longer reachable.
10. As a developer, I want the proxy to handle routing to the correct container port based on the deployment's port configuration, so that I don't need to manually specify upstream addresses.

## Implementation Decisions

- Reverse proxy is embedded in the Lotsen Go process or managed as a controlled subprocess (e.g. Caddy)
- Domain-to-deployment routing table is maintained in memory and persisted by Lotsen
- TLS certificate provisioning uses the ACME protocol via Let's Encrypt
- Certificates are stored on disk and reused across restarts
- HTTP → HTTPS redirect is enabled by default for all domains
- Proxy routing table is updated atomically when a deployment is saved or removed — no restart required
- After a successful zero-downtime redeploy, the proxy upstream is swapped to the new container before the old one is stopped
- Each deployment maps one domain to one container port; multiple domains per deployment are out of scope for v1
- Deployments without a domain are not registered in the proxy routing table and are not reachable externally

## Testing Decisions

- Good tests verify routing behavior and certificate lifecycle, not proxy implementation internals
- Unit tests for routing table management: add domain, update domain, remove domain, no conflicts
- Integration tests: request to a configured domain reaches the correct container
- Integration tests: HTTP request is redirected to HTTPS
- Integration test: after a zero-downtime redeploy, traffic is routed to the new container
- Integration test: removed deployment is no longer reachable via its domain
- Certificate provisioning tested in staging (Let's Encrypt staging environment) to avoid rate limits

## Out of Scope

- Path-based routing (e.g. `/api` → service A, `/app` → service B)
- Multiple domains per deployment
- Custom TLS certificates (bring your own cert)
- DNS configuration or domain registration
- Load balancing across multiple instances of the same container
- WebSocket proxying (may work automatically but not explicitly supported in v1)

## Further Notes

The proxy must be treated as a critical component — it sits in front of all user-facing traffic. Proxy configuration updates must never cause a gap in availability. The routing table swap during a zero-downtime redeploy is the most sensitive operation and must be coordinated carefully with the orchestrator.
