# PRD: Web GUI

## Problem Statement

Managing Docker containers on a VPS today requires SSH access and CLI knowledge. Updating a container means pulling a new image, stopping the old one, running a new one with the right flags — and doing it all without making a mistake. There is no visual overview of what's running, what's healthy, or what the configuration is. This is slow, error-prone, and inaccessible for day-to-day operations.

## Solution

A browser-based GUI served by the Lotsen backend that gives the user full control over their deployments. Users can create, view, edit, and remove deployments, see real-time status and logs, and configure everything needed to run a container in production — all without touching the command line.

## User Stories

1. As a developer, I want to see a list of all my deployments on the home screen, so that I have a clear overview of everything running on my VPS.
2. As a developer, I want to see the status of each deployment (running, deploying, stopped, failed), so that I can quickly identify unhealthy services.
3. As a developer, I want to create a new deployment by filling in a form, so that I can get a container running without writing any CLI commands.
4. As a developer, I want to specify a Docker image and tag when creating a deployment, so that I control exactly which version of the software runs.
5. As a developer, I want to add environment variables to a deployment, so that I can configure my containers without hardcoding values in the image.
6. As a developer, I want to map host ports to container ports, so that I can control how my containers are exposed on the VPS.
7. As a developer, I want to configure volume mounts for a deployment, so that data persists across container restarts and redeployments.
8. As a developer, I want to optionally set a domain on a deployment, so that it becomes publicly reachable via HTTPS without manual proxy configuration.
9. As a developer, I want to edit an existing deployment's configuration and save it, so that I can update my containers without SSH.
10. As a developer, I want saving a changed deployment to automatically trigger a redeployment, so that my changes are applied immediately.
11. As a developer, I want to remove a deployment, so that I can clean up containers I no longer need.
12. As a developer, I want to view real-time logs for a running deployment, so that I can debug issues without opening a terminal.
13. As a developer, I want the deployment status to update in real time in the GUI, so that I can see a redeploy progress without refreshing the page.
14. As a developer, I want to see the last N lines of logs when I open a deployment, so that I have immediate context even for older log entries.
15. As a developer, I want clear error messages when a deployment fails, so that I know what went wrong and how to fix it.

## Implementation Decisions

- React frontend, served as a static build by the Lotsen Go server
- Go backend exposes a REST API for deployment CRUD operations
- Deployment entity fields: id, name, image, envs (key-value pairs), ports (host:container mappings), volumes (host path:container path), domain (optional), status
- Deployment statuses: `idle`, `deploying`, `healthy`, `failed`
- Real-time log streaming via WebSocket or Server-Sent Events (SSE)
- Real-time status updates via WebSocket or SSE
- GUI is served on a well-known port (e.g. 3000 or 8080) by the Lotsen process
- The backend is the single source of truth for deployment state — the GUI is stateless

## Testing Decisions

- Good tests verify API behavior and deployment state transitions, not UI implementation details
- Backend: unit tests for deployment CRUD API handlers
- Backend: integration tests for full deployment lifecycle (create → deploy → edit → redeploy → delete)
- Frontend: component tests for the deployment form (valid/invalid states)
- No end-to-end browser tests for v1

## Out of Scope

- Authentication and access control (v2)
- Multi-user support
- Container template marketplace
- Resource usage graphs (CPU, memory)
- Exec into running containers
- Container restart button (handled automatically by redeploy)

## Further Notes

The GUI should feel fast and minimal. Every interaction should be obvious to a developer who has used Docker before, even if they've never heard of Lotsen. Avoid introducing new terminology where Docker's own terms suffice.
