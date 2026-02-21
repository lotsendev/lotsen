# api

The Dirigent REST API, written in Go. Reads and writes the shared JSON store.

## Requirements

- Go 1.23+

## Development

```bash
# From the api/ directory

# Run the API server (port 8080)
go run ./cmd/dirigent

# Run all tests
go test ./...

# Build the binary
go build -o dirigent ./cmd/dirigent
```

## Configuration

| Environment variable | Default                                   | Description                        |
|----------------------|-------------------------------------------|------------------------------------|
| `DIRIGENT_DATA`      | `/var/lib/dirigent/deployments.json`      | Path to the JSON state file        |

For local development, set `DIRIGENT_DATA` to a writable path:

```bash
DIRIGENT_DATA=/tmp/dirigent.json go run ./cmd/dirigent
```

## API

| Method   | Path                        | Description              |
|----------|-----------------------------|--------------------------|
| `GET`    | `/api/deployments`          | List all deployments     |
| `POST`   | `/api/deployments`          | Create a deployment      |
| `GET`    | `/api/deployments/{id}`     | Get a deployment         |
| `DELETE` | `/api/deployments/{id}`     | Delete a deployment      |

### Deployment object

```json
{
  "id":      "3f2a1b...",
  "name":    "web",
  "image":   "nginx:latest",
  "envs":    { "PORT": "80" },
  "ports":   ["80:80"],
  "volumes": ["/data:/data"],
  "domain":  "example.com",
  "status":  "idle"
}
```

**Status values:** `idle` | `deploying` | `healthy` | `failed`

### Create a deployment

```bash
curl -X POST http://localhost:8080/api/deployments \
  -H "Content-Type: application/json" \
  -d '{"name":"web","image":"nginx:latest","ports":["80:80"]}'
```

## Package structure

```
api/
├── cmd/dirigent/     Entry point
└── internal/
    └── api/          HTTP handlers and Store interface
```

Deployment persistence is provided by the shared `store/` module at the repo root (`github.com/ercadev/dirigent/store`).
