# PRD: Private Docker Registry Support

## Problem
Lotsen currently pulls images with `ImagePull(..., image.PullOptions{})`, which means no registry credentials are sent. Deployments that reference private images fail during reconcile with pull errors.

## Goals
1. Allow deploying images from private registries (Docker Hub private repos, GHCR, ECR, GCR/Artifact Registry, self-hosted registries).
2. Keep credentials out of logs and API responses.
3. Preserve current UX for public images (no extra setup required).
4. Support safe credential rotation without recreating deployments.

## Non-goals
- Full external secret-manager integration in v1.
- Node-level credential helpers (`~/.docker/config.json`) management by Lotsen.

## Current architecture touchpoints
- Deployment model is stored in `store.Deployment` and currently has no registry auth fields.
- API create/update payloads map to the deployment model and currently do not accept registry credentials.
- Orchestrator image pulls happen in `pullImage`, which currently uses empty `image.PullOptions`.

## Proposed design

### 1) Data model
Add optional registry auth metadata to deployments:

```go
type RegistryAuth struct {
    ServerAddress string `json:"server_address"` // e.g. ghcr.io, index.docker.io
    Username      string `json:"username,omitempty"`
    Password      string `json:"password,omitempty"`
    IdentityToken string `json:"identity_token,omitempty"` // cloud/OIDC style token
}
```

Add to `store.Deployment`:

```go
RegistryAuth *RegistryAuth `json:"registry_auth,omitempty"`
```

Validation rules:
- `server_address` required when `registry_auth` is set.
- Require either:
  - `username` + `password`, or
  - `identity_token`.
- Trim whitespace and reject empty values.

### 2) API contract
Update API request payloads (`deploymentRequest`, `patchDeploymentRequest`) to accept `registry_auth`.

Security response rules:
- Never echo `password` or `identity_token` from read endpoints.
- Return redacted shape in `GET` responses, e.g.:

```json
"registry_auth": {
  "server_address": "ghcr.io",
  "username": "my-user",
  "configured": true
}
```

This keeps UI state understandable without exposing secrets.

### 3) Secure persistence
v1 approach:
- Persist encrypted registry secret fields at rest using an app-level key from env (e.g. `LOTSEN_SECRET_KEY`).
- Encrypt only secret fields (`password`, `identity_token`), keep non-secret metadata plaintext.

Operational behavior:
- If key is missing and encrypted data exists, fail startup with clear error.
- Add migration handling for deployments without `registry_auth`.

### 4) Orchestrator pull flow
Change pull logic to pass Docker registry auth:

1. Build `dockertypes.AuthConfig` from deployment `registry_auth`.
2. Base64url-encode JSON auth config as required by Docker API.
3. Set `image.PullOptions{RegistryAuth: encoded}`.
4. Keep current retry/timeouts behavior.

Behavior:
- Public images continue using empty auth.
- Private pulls use deployment-specific credentials.
- Errors should be normalized to actionable messages (401/denied → "invalid registry credentials", not generic pull failure).

### 5) Dashboard UX
Create/Edit deployment forms should include optional "Private registry" section:
- Registry server
- Username/password OR token
- "Update credentials" workflow that allows rotating without exposing current secret

UI read-path should rely on redacted API response (`configured=true`) and never require returning plaintext secrets.

### 6) Logging and redaction
Ensure these values are redacted everywhere:
- API logs
- Reconcile logs
- Event streams / status error messages

Add helper `redactRegistryAuth(err/error context)` patterns where pull errors may include request metadata.

## API examples

### Create deployment with private image
```json
{
  "name": "api",
  "image": "ghcr.io/acme/private-api:1.4.0",
  "ports": ["8080:8080"],
  "registry_auth": {
    "server_address": "ghcr.io",
    "username": "acme-ci",
    "password": "<secret>"
  }
}
```

### Rotate registry token via patch
```json
{
  "registry_auth": {
    "server_address": "123456789.dkr.ecr.us-east-1.amazonaws.com",
    "identity_token": "<new-token>"
  }
}
```

## Implementation plan
1. **Model + validation**
   - Add `RegistryAuth` structs in store/API layer.
   - Add strict validation and normalization.
2. **Persistence + encryption**
   - Add crypto helpers and key bootstrap.
   - Update JSON marshal/unmarshal paths.
3. **Orchestrator integration**
   - Extend deployment object consumed by orchestrator.
   - Pass `RegistryAuth` in `ImagePull` options.
4. **Response redaction**
   - Ensure list/get/create/update responses never leak secrets.
5. **Dashboard form support**
   - Add create/edit controls and redact-aware state.
6. **Docs**
   - Update deployment configuration docs with private registry section.

## Testing strategy

### Unit tests
- API validation for all credential combinations.
- API read response redaction assertions.
- Crypto roundtrip tests for persisted secret fields.
- Orchestrator tests ensuring `ImagePull` receives expected auth payload.

### Integration tests
- Local registry with auth (`registry:2` + htpasswd) in CI smoke test.
- Successful deploy from private image.
- Failure path with bad credentials shows clear deployment error.

## Rollout
1. Release behind a feature flag (`LOTSEN_PRIVATE_REGISTRY_ENABLED=true`) for one release.
2. Remove flag after stabilization.
3. Backward-compatible: existing deployments unaffected.

## Open questions
1. Should we store one credential set per deployment (simple) or support reusable named registry credentials (better UX, more complexity)?
2. Do we want registry credential test/validate endpoint in the API before saving?
3. For cloud registries (ECR/GCR), should Lotsen manage token refresh in v2?
