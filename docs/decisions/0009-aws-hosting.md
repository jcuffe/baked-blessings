# 0009 — AWS for all hosting; pod-ready containers built in CI

**Status:** accepted 2026-11-23 (supersedes the hosting parts of 0001, 0005 and 0006)

## Context
Jared runs AWS with Pulumi at work and wants one cloud provider so the IaC stays simple. The long-term home is his own EKS cluster, with this store as one application among many. Until revenue supports that, hosting should use the cheapest AWS products (Lambda, Fargate). Shopify is treated like a library vendor: we use it where it is the market leader or best value (checkout and card payments, point of sale), not for hosting.

## Decision
- **Every app ships as a container image built in CI** (`apps/<app>/Dockerfile`), with a `/healthz` endpoint, config from environment variables and graceful shutdown on SIGTERM, so the same image runs on Lambda, Fargate or a Kubernetes pod.
- **Storefront:** the Hydrogen build (the same worker bundle Oxygen runs) served by a small Node server (`apps/storefront/container/server.mjs`). On Lambda it runs through the AWS Lambda Web Adapter with a streaming function URL, behind CloudFront. Its `*.cloudfront.net` address is the public link until a domain is bought. Images live in Amazon ECR; CI pushes them through a GitHub OIDC role, so GitHub holds no AWS keys.
- **IaC:** `infra/` (Pulumi Go) gains the AWS provider. `infra/internal/storefront` owns the storefront's AWS resources.
- **Oxygen** stays only as the preview environment on the Partner dev store, because it is free and gives every branch its own URL.
- **Bakehouse, Postgres and DNS** move from Render and Cloudflare to AWS in a follow-up (Fargate or Lambda, RDS or Aurora Serverless, Route 53). Until then the Render and Cloudflare parts of `infra/` only run when the stack sets `domain`.

## Costs (us-east-1 list prices, read 2026-11-22)
- Storefront on Lambda + CloudFront: about $0 a month at launch traffic. Lambda's always-free tier is 1M requests and 400,000 GB-seconds a month; CloudFront's is 1 TB out and 10M requests; ECR storage is $0.10 per GB-month (a few cents for ten images).
- The same storefront always-on on Fargate behind an ALB: about $33 a month (0.25 vCPU ARM task about $7, public IPv4 about $3.65, ALB about $16 plus LCUs). Move there when cold starts or traffic justify it; EKS adds $73 a month for the control plane.
- Route 53, once there is a domain: $0.50 per hosted zone a month.

## Consequences
- Checkout: at the time of this decision, real payments still needed a paid Shopify plan. [ADR 0010](0010-stripe-checkout-own-catalog.md) replaces Shopify checkout with Stripe Checkout, so no Shopify plan is needed.
- Cold starts on Lambda add latency to the first request after a quiet spell. Provisioned concurrency or Fargate removes it, at a cost.
- `dns.go` still points the apex at Shopify's storefront IPs from the Oxygen plan; it changes when DNS moves to Route 53.
