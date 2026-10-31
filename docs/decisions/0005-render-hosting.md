# 0005 — Bakehouse and Postgres on Render

**Status:** accepted 2026-10-10 (supersedes the initial Fly.io + Neon choice)

## Context
Bakehouse needs an always-on Node process (web + worker) and Postgres, cheaply, with infrastructure-as-code support. Fly.io's Terraform provider is archived and its Pulumi provider is stale; Neon's free tier cannot sustain an always-polling job queue.

## Decision
Run Bakehouse as a Render Starter web service from a Docker image in GHCR, with the worker in-process for v1 (`WORKER_MODE=inline`), and Render Postgres (basic) in Oregon. About $14.50/month.

## Consequences
- Managed through Render's official Terraform provider via Pulumi's bridge (`infra/sdks/render`).
- Docker images keep a move to Hetzner or DigitalOcean cheap.
- The worker can be split into its own Render service later.
