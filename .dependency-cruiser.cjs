// Architecture boundaries (docs/SPEC.md, ADR 0004). The core packages hold our
// differentiating logic and must stay portable off Shopify and off React Router.
const CORE = "^packages/(bake-domain|ledger|contracts)/";

/** @type {import('dependency-cruiser').IConfiguration} */
module.exports = {
  forbidden: [
    {
      name: "core-no-shopify",
      comment: "Core packages must not depend on Shopify SDKs; use a port + adapter.",
      severity: "error",
      from: { path: CORE },
      to: { path: "@shopify/" },
    },
    {
      name: "core-no-framework",
      comment: "Core packages must not depend on React or React Router.",
      severity: "error",
      from: { path: CORE },
      to: { path: "(^|/)node_modules/(react|react-dom|react-router|@react-router/)" },
    },
    {
      name: "core-no-apps",
      comment: "Core packages must not import from apps.",
      severity: "error",
      from: { path: CORE },
      to: { path: "^apps/" },
    },
    {
      name: "domain-is-pure",
      comment: "bake-domain is pure logic: no database, network or provider clients.",
      severity: "error",
      from: { path: "^packages/bake-domain/" },
      to: { path: "(^|/)node_modules/(drizzle-orm|pg|zod|resend|twilio)" },
    },
  ],
  options: {
    doNotFollow: { path: "node_modules" },
    exclude: { path: "(^|/)(dist|build|\\.react-router|node_modules)/" },
    tsPreCompilationDeps: true,
    combinedDependencies: true,
  },
};
