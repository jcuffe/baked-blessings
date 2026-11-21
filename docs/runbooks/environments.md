# Environments: dev, preview and production

Where each version of the site runs, and how to look at it. Laptop setup is in [local-development.md](local-development.md); everything there also works inside a codespace.

| Environment | Where | Who can open it | Cost |
|---|---|---|---|
| Dev | GitHub Codespaces, `pnpm dev` | You, through GitHub's forwarded-port URL | Free tier: 120 core-hours (60 hours on 2 cores) and 15 GB a month on GitHub Free |
| Preview | Oxygen, on the Partner dev store | Store staff (log in to the dev store) | Free |
| Production | Oxygen, on Jakki's store, public | Anyone with the link | Shopify Basic plan; domain at cost on Cloudflare |

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
Every push to a branch other than `master` that touches the storefront runs `.github/workflows/oxygen.yml`, which deploys to the dev store's Oxygen. Branches without an Oxygen environment of their own land in **Preview**, and every deployment gets its own URL.

- Find the URL in the workflow run's log, or in Shopify admin → **Hydrogen → Baked Blessings → Deployments**. With "Comment on pull requests with deployment preview links" on (Storefront settings → Oxygen deployments) it is also posted on the PR.
- Dev-store URLs always need a store login, so open them while signed in to the dev store. Shareable "anyone with the link" previews need a paid plan, so they're only available on Jakki's store.

One-time setup:
1. Create the Partner dev store and install the Hydrogen channel ([local-development.md](local-development.md#connect-to-shopify-one-time)).
2. In the dev store: **Hydrogen → storefront → Storefront settings → Oxygen deployment tokens → Generate**.
3. Save it as `SHOPIFY_HYDROGEN_DEPLOYMENT_TOKEN` in the `staging` GitHub environment (by hand, or through `infra/`).

## Production: Oxygen on Jakki's store
Pushes to `master` deploy through the `prod` GitHub environment to Jakki's store, where Oxygen maps the default branch to **Production**.

One-time setup:
1. Jakki's store on the Basic plan, with the Hydrogen channel installed and the storefront linked.
2. A deployment token from that store, saved as `SHOPIFY_HYDROGEN_DEPLOYMENT_TOKEN` in the `prod` GitHub environment. Add required reviewers to that environment if production deploys should wait for approval.
3. **Storefront settings → Environments and variables → Production → URL privacy → Public.** The environment's Oxygen URL is now a shareable https link.
4. Custom domain, once bought on Cloudflare: the `infra` prod stack writes the records Shopify asks for (A and AAAA on the apex, `www` CNAME to `shops.myshopify.com`). Then in Shopify **Settings → Domains**, connect it, target the Hydrogen production storefront, and make it primary.

## Reviewing a change
1. Work on a branch in a codespace (or have Claude Code do it) and click through on the forwarded Storefront port while it's in progress.
2. Push. Open the Oxygen preview for that commit and review look, feel and behavior against the store's real data.
3. Merge to `master`. Check the production URL once the deploy finishes; roll back from Shopify admin → Deployments if needed.
