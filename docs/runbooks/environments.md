# Environments: dev, preview and production

Where each version of the site runs, and how to look at it. Laptop setup is in [local-development.md](local-development.md); everything there also works inside a codespace.

| Environment | Where | Who can open it | Cost |
|---|---|---|---|
| Dev | GitHub Codespaces, `pnpm dev` | You, through GitHub's forwarded-port URL | Free tier: 120 core-hours (60 hours on 2 cores) and 15 GB a month on GitHub Free |
| Preview | Oxygen, on the Partner dev store | Store staff (log in to the dev store) | Free |
| Production | AWS: Lambda behind CloudFront, from the container CI builds | Anyone with the link (`https://<id>.cloudfront.net` until there's a domain) | About $0 a month at launch traffic; real checkout needs a paid Shopify plan |

## Dev: GitHub Codespaces
1. On GitHub, **Code → Codespaces → Create codespace on `<branch>`**. Keep the default 2-core machine.
2. The first build runs `pnpm bootstrap` (dependencies, `.env` files, Postgres in Docker, migrations). Later starts reuse it.
3. Run the stack:

   ```bash
   pnpm dev
   ```

4. Open the **Ports** tab and click the globe next to **Storefront (3000)**. The URL looks like `https://<codespace>-3000.app.github.dev` and only your GitHub login can open it. To show someone, right-click the port and set visibility to **Public**; set it back when done.
5. Claude Code is installed. Run `claude` in the terminal (or open the Claude Code panel) and sign in once per codespace.

Keeping inside the free tier:
- Stop the codespace when you're done (**Codespaces → … → Stop**). Set the idle timeout to 30 minutes or less in [your Codespaces settings](https://github.com/settings/codespaces).
- Delete codespaces for merged branches; each one holds its own copy of `node_modules` and the database.
- In [billing budgets](https://github.com/settings/billing/budgets), add a $0 Codespaces budget with **Stop usage when budget limit is reached**, so going over the quota stops the codespace instead of charging.

Shopify-connected mode (`pnpm dev:shopify`) works the same way in a codespace; the Shopify CLI opens its own tunnel.

## Preview: Oxygen on the dev store
Every push that touches the storefront runs `.github/workflows/oxygen.yml`, which deploys to the dev store's Oxygen. Branches without an Oxygen environment of their own land in **Preview**, and every deployment gets its own URL.

- Find the URL in the workflow run's log, or in Shopify admin → **Hydrogen → Baked Blessings → Deployments**. With "Comment on pull requests with deployment preview links" on (Storefront settings → Oxygen deployments) it is also posted on the PR.
- Dev-store URLs always need a store login, so open them while signed in to the dev store. Shareable "anyone with the link" Oxygen URLs need a paid plan; the public link is production on AWS.

One-time setup:
1. Create the Partner dev store and install the Hydrogen channel ([local-development.md](local-development.md#connect-to-shopify-one-time)).
2. In the dev store: **Hydrogen → storefront → Storefront settings → Oxygen deployment tokens → Generate**.
3. Save it as `SHOPIFY_HYDROGEN_DEPLOYMENT_TOKEN` in the `staging` GitHub environment (by hand, or through `infra/`).

## Production: AWS (Lambda + CloudFront)
The storefront ships as a container (`apps/storefront/Dockerfile`): the same Hydrogen build Oxygen runs, served by plain Node. `.github/workflows/storefront-image.yml` builds it on every change and checks that it serves pages. On `master` it pushes the image to Amazon ECR and rolls the production Lambda onto it, through the `prod` GitHub environment. Why AWS and what it costs: [ADR 0009](../decisions/0009-aws-hosting.md).

Run the image locally (needs Docker; works in a codespace):

```bash
docker build -f apps/storefront/Dockerfile -t storefront .
```

```bash
docker run --rm -p 8080:8080 -e SESSION_SECRET=dev -e PUBLIC_STORE_DOMAIN=mock.shop storefront
```

One-time setup (each step creates AWS resources, so it waits for a yes):
1. An AWS account, and credentials for Pulumi on the machine that runs it (`aws configure sso` or an access key).
2. In `infra/`, on the `prod` stack:

   ```bash
   pulumi config set aws:region us-east-1
   ```

   ```bash
   pulumi config set githubRepository jcuffe/baked-blessings
   ```

   If the AWS account already has a GitHub OIDC provider, also set `githubOidcProviderArn` to its ARN.
3. `pulumi up -s prod`. This first run creates only the ECR repository and the CI role, and writes `AWS_REGION`, `AWS_ROLE_ARN` and `STOREFRONT_ECR_REPOSITORY_URL` into the `prod` GitHub environment (needs `github:token`).
4. Re-run the **Storefront image** workflow on `master` (Actions → Storefront image → Run workflow). It pushes an image tagged with the commit SHA.
5. Set the storefront's settings and the first image, then apply again:

   ```bash
   pulumi config set storefrontImageTag <commit-sha>
   ```

   ```bash
   pulumi config set PUBLIC_STORE_DOMAIN <store>.myshopify.com
   ```

   ```bash
   pulumi config set --secret SESSION_SECRET "$(openssl rand -hex 32)"
   ```

   Add `PUBLIC_STOREFRONT_API_TOKEN`, `PUBLIC_STOREFRONT_ID`, `PUBLIC_CHECKOUT_DOMAIN` and the customer-account values from the Hydrogen channel (`pnpm shopify hydrogen env pull` shows them), and `PRIVATE_STOREFRONT_API_TOKEN` with `--secret`. Then `pulumi up -s prod`. The `storefrontUrl` output is the public link.
6. From then on, every merge to `master` deploys by itself. Roll back by re-running the workflow on an older commit.

Checkout: the storefront still runs the Hydrogen scaffold against the Shopify dev store, so it can be browsed but not paid for. Payments move to Stripe Checkout with our own catalog and admin ([ADR 0010](../decisions/0010-stripe-checkout-own-catalog.md)); no Shopify plan is needed.

Custom domain, later: Route 53 and an ACM certificate on the CloudFront distribution (follow-up; ADR 0009).

## Reviewing a change
1. Work on a branch in a codespace (or have Claude Code do it) and click through on the forwarded Storefront port while it's in progress.
2. Push. Open the Oxygen preview for that commit and review look, feel and behavior against the store's real data.
3. Merge to `master`. Check the production link once the **Storefront image** workflow finishes; roll back by re-running it on an older commit.
