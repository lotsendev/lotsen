# Product Definition: Lotsen

## Vision
Give any solo developer or small team the power of a production-grade Docker orchestration platform without the complexity tax of Kubernetes.

## Problem
Running Docker containers in production on a VPS is manual, fragile, and undocumented. Kubernetes solves this but at a cost — steep learning curve, expensive infrastructure, and operational overhead that solo developers and small teams can't justify. Existing tools (Coolify, CapRover, Portainer) have grown complex and opinionated. There is no tool that simply does the job and stays out of the way.

## Target User
Solo developers and small engineering teams (1–5 people) running production workloads on one or a few VPS instances. They know Docker, they don't want to learn Kubernetes, and they value their time over configurability.

## Core Value Proposition
Lotsen is the simplest path from "a VPS with Docker" to "production-ready container orchestration." No YAML sprawl, no cluster setup, no ops expertise required. Install in one command, deploy in minutes.

## Key Features (v1)
- **One-script installer**: Full setup in a single command. Zero prerequisites beyond Docker.
- **Web GUI**: Deploy, edit, and remove containers from a browser. No SSH required for day-to-day ops.
- **Zero-downtime deployments**: Rolling restarts so production never goes down during updates.
- **Integrated reverse proxy / load balancer**: Expose containers to the internet without manually configuring nginx or Caddy.

## Out of Scope (v1)
- **GitOps deployment**: Deferred to v2 — GUI is the primary path first.
- **Multi-server / cluster support**: Single-VPS only.
- **Team permissions / RBAC**: Single-owner model; no user roles.
- **Secrets management**: Environment variables only; no vault integration.
- **App marketplace / templates**: No one-click app catalog.

## Success Metrics
- Lotsen is running in at least one personal production environment (dogfood signal)
- 100+ installs within 60 days of public release
- End-to-end time from blank VPS to running container is under 5 minutes
