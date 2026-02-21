# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Dirigent is a Docker container orchestration tool for solo developers and small teams running production workloads on a VPS. It is positioned as a lightweight alternative to Kubernetes, offering:

- One-script installer
- Web GUI for deploying/editing/removing Docker containers
- GitOps-based deployment alternative
- Zero-downtime Docker deployments
- Integrated load balancer / reverse proxy

## Tech Stack

- **Backend:** Go (Docker orchestrator)
- **Frontend:** React (web GUI)
- **Infrastructure:** Docker, VPS

## Repository Status

This project is in early development. As source code is added, update this file with:

- Build commands (`go build`, `npm run build`, etc.)
- How to run tests (`go test ./...`, `npm test`, etc.)
- How to run linters
- Architecture notes covering how the Go backend, React frontend, load balancer, and GitOps components interact
