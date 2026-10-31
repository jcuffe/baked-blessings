# 0006 — Pulumi (Go) for infrastructure; Cloudflare DNS

**Status:** accepted 2026-10-10

## Context
Infrastructure will be assembled from the cheapest providers, so the IaC tool must span many providers; Jared wants it in Go. CDKTF is being retired; Pulumi supports Go natively and can bridge any Terraform provider.

## Decision
`infra/` is a Pulumi Go program with `staging` and `prod` stacks, state in Pulumi Cloud Free. It manages Cloudflare DNS and Email Routing, Render services and Postgres, and GitHub Actions environments. The domain is bought by hand on Cloudflare Registrar (at cost). Resend and Twilio have no dependable providers and are set up manually from runbooks; their DNS records live in Pulumi.

## Consequences
- Domain-wide records (storefront, email) are owned only by the prod stack.
- Bootstrap secrets for CI (Pulumi and provider tokens) are set by hand.
