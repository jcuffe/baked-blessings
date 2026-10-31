# Jakki's Baked Blessings

Online storefront for Jakki's home-baked sourdough (Lincoln, CA). Customers order for local pickup or delivery and pay by card through Shopify checkout; bakes, loaf limits and order lists are managed from the Shopify admin.

- **Spec:** [docs/SPEC.md](docs/SPEC.md) · **Decisions:** [docs/decisions/](docs/decisions/) · **Compliance:** [docs/compliance.md](docs/compliance.md)

## Layout
| Path | What |
|---|---|
| `apps/storefront` | Hydrogen storefront (React Router 7) on Oxygen |
| `apps/bakehouse` | Embedded Shopify admin app + storefront API + webhooks + worker, on Render |
| `packages/bake-domain` | Pure bake rules: cadence, cutoffs, capacity math |
| `packages/ledger` | Capacity ledger services + Postgres repositories |
| `packages/contracts` | zod schemas for the storefront ↔ Bakehouse API |
| `packages/brand` | Design tokens (`tokens.css`) and brand assets |
| `infra` | Pulumi (Go): Cloudflare DNS, Render, GitHub environments |

Core packages may not import Shopify, React or React Router — `pnpm boundaries` enforces it.

## Getting started
Install the tools pinned in `.mise.toml` (`mise install`), start Docker, then:

```bash
pnpm bootstrap
```

```bash
pnpm dev
```

The storefront is at http://localhost:3000 (mock.shop data until linked) and Bakehouse at http://localhost:3001/healthz. Run `pnpm check` before pushing.

[docs/runbooks/local-development.md](docs/runbooks/local-development.md) covers each component, Shopify-connected mode (`pnpm dev:shopify`), database and env-var routines, the one-time Shopify setup and troubleshooting. Infrastructure: [infra/README.md](infra/README.md).
