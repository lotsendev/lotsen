# Dirigent

A lightweight Docker orchestration tool for solo developers and small teams running production workloads on a VPS — a simpler alternative to Kubernetes.

## Features

- One-script installer, up and running fast
- Web dashboard to deploy, edit, and remove Docker containers
- GitOps-based deployments as an alternative workflow
- Zero-downtime rolling deployments
- Integrated load balancer / reverse proxy

## Why?

- Managing Docker containers on a VPS today is painful
- Kubernetes is overkill and expensive for solo developers and small teams

## Monorepo structure

| Directory        | Description                                  |
|------------------|----------------------------------------------|
| `control-plane/` | Go orchestration engine + REST API (`:8080`) |
| `dashboard/`     | React + Vite web dashboard (`:3000`)         |

See each directory's README for development instructions.

## Tech stack

- **control-plane:** Go
- **dashboard:** React, Vite, Bun
