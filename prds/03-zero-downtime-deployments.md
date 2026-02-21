# PRD: Zero-Downtime Deployments

## Problem Statement

Updating a Docker container in production — changing the image version, updating an env var, modifying a port — requires stopping the running container before starting the new one. This causes downtime. For any production service, even a few seconds of unavailability during a routine update is unacceptable.

## Solution

When a deployment's configuration changes and the user saves, Dirigent starts the new container alongside the old one. Once the new container is confirmed healthy, the old one is stopped. If the new container never becomes healthy, the old one continues running uninterrupted. The user sees the deployment status update in real time throughout the process.

## User Stories

1. As a developer, I want my service to remain available while I update a deployment, so that users don't experience downtime during routine changes.
2. As a developer, I want Dirigent to start the new container before stopping the old one, so that there's always a healthy instance serving traffic.
3. As a developer, I want Dirigent to use a Docker healthcheck if one is defined in the image, so that the new container is only considered healthy when it's actually ready to serve requests.
4. As a developer, I want Dirigent to fall back to checking that the container stays running for a short period if no healthcheck is defined, so that basic stability is verified even for images without explicit healthchecks.
5. As a developer, I want the old container to keep running if the new one never becomes healthy, so that my service stays up even during a bad deployment.
6. As a developer, I want to see the deployment status change to "deploying" when a save triggers a redeploy, so that I know the process has started.
7. As a developer, I want to see the deployment status update to "healthy" or "failed" once the redeploy completes, so that I know the outcome without having to check manually.
8. As a developer, I want redeployment to trigger automatically when I save changes to a deployment's configuration, so that I don't need to press a separate deploy button.
9. As a developer, I want the reverse proxy to route traffic to the new container only after it's healthy, so that users are never routed to a container that isn't ready.
10. As a developer, I want the old container to be stopped and cleaned up after a successful redeploy, so that I don't accumulate stale containers on my VPS.

## Implementation Decisions

- Orchestrator module in Go responsible for the full deployment lifecycle
- Redeploy sequence: pull new image → start new container → wait for healthy → update proxy routing → stop old container → clean up old container
- Health check strategy:
  - If Docker healthcheck is defined: poll until status is `healthy` or timeout is reached
  - If no healthcheck: wait for container to be running without exiting for a configurable grace period (e.g. 10 seconds)
- If health check fails or times out: leave old container running, mark deployment as `failed`, clean up new container
- Deployment statuses managed by orchestrator: `idle`, `deploying`, `healthy`, `failed`
- Reverse proxy routing table updated atomically after new container passes health check
- Only one redeployment per deployment can be in progress at a time — concurrent saves are queued or rejected
- Health check timeout and grace period are sensible defaults, not user-configurable in v1

## Testing Decisions

- Good tests verify observable state transitions and outcomes, not internal orchestrator implementation
- Unit tests for health check logic: Docker healthcheck polling, fallback grace period, timeout handling
- Integration tests for full redeploy lifecycle: old container stays up during deploy, new container takes over after healthy, old container removed after success
- Integration test for failure case: new container fails health check, old container remains running, deployment marked failed
- Tests use real Docker (not mocked) to ensure container lifecycle behavior is correct

## Out of Scope

- Manual rollback button (v2)
- Custom HTTP health check endpoints defined by the user (v2)
- Canary or weighted traffic splitting
- Deployment history / audit log
- Concurrent redeployments of the same deployment

## Further Notes

The key invariant is: at no point during a redeploy should a request to the domain fail due to Dirigent's actions. The old container is the fallback and must stay alive until the new one is confirmed good. This should be treated as a safety-critical property of the orchestrator.
