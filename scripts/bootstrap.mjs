// One-command local setup: `pnpm bootstrap`. Safe to re-run at any time; it never
// overwrites a value you have set. See docs/runbooks/local-development.md.
import { spawnSync } from "node:child_process";
import { randomBytes } from "node:crypto";
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { parseEnv } from "node:util";

const root = join(import.meta.dirname, "..");
const pkg = JSON.parse(readFileSync(join(root, "package.json"), "utf8"));

// Apps with an .env.example, and the keys that get a random value when empty.
const ENV_FILES = [
  { dir: "apps/storefront", generate: ["SESSION_SECRET"] },
  { dir: "apps/bakehouse", generate: [] },
];

function step(title) {
  console.log(`\n▸ ${title}`);
}

function run(cmd, args) {
  const result = spawnSync(cmd, args, { cwd: root, stdio: "inherit" });
  if (result.status !== 0) {
    console.error(`\n✗ ${cmd} ${args.join(" ")} failed`);
    process.exit(result.status ?? 1);
  }
}

function checkTools() {
  step("Checking tools");
  const [major, minor] = process.versions.node.split(".").map(Number);
  if (major < 22 || (major === 22 && minor < 12)) {
    console.error(`✗ Node ${process.versions.node} is too old; this repo needs ${pkg.engines.node}. Run \`mise install\`.`);
    process.exit(1);
  }
  if (major !== 22) {
    console.warn(`! Node ${process.versions.node}: CI and Render use Node 22 (.mise.toml).`);
  }

  const wantPnpm = pkg.packageManager.split("@")[1];
  const havePnpm = /pnpm\/(\S+)/.exec(process.env.npm_config_user_agent ?? "")?.[1];
  if (havePnpm && havePnpm !== wantPnpm) {
    console.warn(`! pnpm ${havePnpm}: this repo pins ${wantPnpm}. Run \`mise install\` or \`corepack enable\`.`);
  }

  if (spawnSync("docker", ["info"], { stdio: "ignore" }).status !== 0) {
    console.error("✗ Docker is not running. Start Docker Desktop (or OrbStack/colima) and re-run.");
    process.exit(1);
  }
  console.log(`  node ${process.versions.node}, pnpm ${havePnpm ?? "?"}, docker ok`);
}

// Copies .env.example to .env, then fills in keys that are missing or should be
// generated. Existing values are left alone.
function syncEnvFile({ dir, generate }) {
  const examplePath = join(root, dir, ".env.example");
  const envPath = join(root, dir, ".env");
  const example = readFileSync(examplePath, "utf8");
  const created = !existsSync(envPath);
  let text = created ? example : readFileSync(envPath, "utf8");
  const current = parseEnv(text);
  const changes = [];

  const defaults = parseEnv(example);
  for (const [, key] of example.matchAll(/^([A-Za-z_]\w*)=/gm)) {
    if (!(key in current)) {
      text = `${text.trimEnd()}\n${key}=${defaults[key]}\n`;
      current[key] = defaults[key];
      if (!created) changes.push(`added ${key}`);
    }
  }
  for (const key of generate) {
    if (!current[key]) {
      const line = `${key}=${randomBytes(32).toString("hex")}`;
      text = text.replace(new RegExp(`^${key}=.*$`, "m"), line);
      changes.push(`generated ${key}`);
    }
  }

  if (created || changes.length > 0) writeFileSync(envPath, text);
  const summary = created ? "created from .env.example" : changes.join(", ") || "up to date";
  console.log(`  ${dir}/.env: ${summary}`);
}

checkTools();

step("Installing dependencies");
run("pnpm", ["install"]);

step("Writing .env files");
ENV_FILES.forEach(syncEnvFile);

step("Starting Postgres");
run("pnpm", ["db:up"]);

step("Building shared packages");
run("pnpm", ["turbo", "run", "build", "--filter=./packages/*"]);

step("Applying database migrations");
run("pnpm", ["--filter", "@bb/bakehouse", "db:migrate"]);

console.log(`
✓ Ready. Start everything with:

    pnpm dev

  Storefront  http://localhost:3000  (mock.shop data until linked)
  Bakehouse   http://localhost:3001/healthz
  Postgres    localhost:55432  (pnpm db:psql)

See docs/runbooks/local-development.md for Shopify-connected mode.`);
