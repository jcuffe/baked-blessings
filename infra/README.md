# Infrastructure (Pulumi, Go)

Manages Cloudflare DNS + Email Routing, Bakehouse on Render (web service + Postgres), and GitHub Actions environments. Stacks: `staging`, `prod`. State: Pulumi Cloud (free tier).

The Render provider is bridged from Terraform (`render-oss/render` 1.9.1) and its SDK is generated into `sdks/render`. Regenerate after bumping the version in `Pulumi.yaml`:

```bash
pulumi package add terraform-provider render-oss/render 1.9.1
```

## Manual prerequisites
- Domain bought on Cloudflare Registrar (the zone is looked up, not created).
- API tokens: Cloudflare (Zone DNS edit, Email Routing edit), Render API key + owner ID, GitHub token (repo admin) — and a Pulumi access token for CI.
- Resend: add the domain in the dashboard and copy its DKIM value into `resendDkim`.
- Twilio: buy a number, create a Messaging Service, complete A2P 10DLC.
- After the first `pulumi up` with `forwardTo`, click the verification link Cloudflare emails to that address.

## Stack setup
```bash
pulumi login
```

```bash
pulumi stack init staging
```

Then set configuration (repeat per stack; `--secret` for secrets):

| Key | Example / notes |
|---|---|
| `domain` | `jakkisbakedblessings.com` |
| `region` | `oregon` (default) |
| `bakehouseImage`, `bakehouseImageTag` | `ghcr.io/jcuffe/baked-blessings-bakehouse`, `latest` |
| `bakehousePlan`, `postgresPlan`, `postgresVersion` | `starter`, `basic_256mb`, `17` (defaults) |
| `ghcrUsername` + `ghcrToken` (secret) | only if the image is private |
| `SHOPIFY_API_KEY`, `SHOP_DOMAIN`, `SCOPES` | from the linked app / store |
| `SHOPIFY_API_SECRET`, `STOREFRONT_HMAC_SECRETS`, `SLOT_SIGNING_SECRET` | secrets (required) |
| `RESEND_API_KEY`, `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_MESSAGING_SERVICE_SID` | secrets (optional) |
| `resendDkim`, `forwardTo` (secret) | prod only: Resend DKIM value; inbox for `hello@` |
| `githubRepository` + `SHOPIFY_HYDROGEN_DEPLOYMENT_TOKEN`, `SHOPIFY_CLI_PARTNERS_TOKEN` (secrets) | optional CI environment |
| `cloudflare:apiToken`, `render:apiKey`, `render:ownerId`, `github:token`, `github:owner` | provider config (secrets) |

Example:

```bash
pulumi config set --secret SHOPIFY_API_SECRET
```

Preview and apply:

```bash
pulumi preview -s staging
```

```bash
pulumi up -s staging
```

The prod stack owns the domain-wide records (storefront A/AAAA/www, Resend, DMARC, Email Routing); staging only adds `bakehouse.staging.<domain>`. The prod Postgres is `protect`ed against accidental deletion.

## Tests
```bash
go test ./...
```
