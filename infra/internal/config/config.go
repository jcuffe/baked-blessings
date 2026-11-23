// Package config reads stack configuration for the baked-blessings infrastructure.
package config

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pcfg "github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Config is the typed view of Pulumi.<stack>.yaml.
type Config struct {
	// Stack is "staging" or "prod".
	Stack  string
	IsProd bool

	// Domain is the Cloudflare zone, e.g. "jakkisbakedblessings.com". Bought by hand.
	// Optional: without it the stack skips Cloudflare DNS and Bakehouse on Render, which
	// are on their way to AWS (docs/decisions/0009-aws-hosting.md).
	Domain string
	// HasRender is true when the stack manages Bakehouse and Postgres on Render.
	HasRender bool
	// Region is the Render region for Bakehouse and Postgres.
	Region string

	BakehouseImage    string
	BakehouseImageTag string
	BakehousePlan     string
	PostgresPlan      string
	PostgresVersion   string

	// Optional: GHCR token for pulling a private image.
	GhcrUsername string
	GhcrToken    pulumi.StringOutput

	// Plain Bakehouse environment.
	ShopifyAPIKey string
	ShopDomain    string
	Scopes        string

	// Secret Bakehouse environment, keyed by env var name. Missing optional keys are skipped.
	BakehouseSecrets map[string]pulumi.StringOutput

	// Optional email setup (prod only): Cloudflare Email Routing and Resend DKIM.
	ForwardTo  pulumi.StringOutput
	HasForward bool
	ResendDKIM string

	// Optional: "owner/repo" whose Actions environment receives CI secrets.
	GitHubRepository string
	CISecrets        map[string]pulumi.StringOutput

	// Storefront on AWS. Enabled by setting aws:region on the stack.
	Storefront Storefront
}

// Storefront configures the containerized storefront on AWS (Lambda behind CloudFront).
type Storefront struct {
	Enabled bool
	// Region is the AWS region (aws:region).
	Region string
	// ImageTag is the tag in the stack's ECR repository that the Lambda is created from.
	// Leave it empty on the first `pulumi up`: that creates the repository and the CI
	// role, CI pushes an image, and the next `pulumi up` with the tag creates the Lambda
	// and CloudFront. CI rolls later images onto the Lambda itself.
	ImageTag string
	// MemoryMB sizes the Lambda; CPU scales with it.
	MemoryMB int
	// GitHubOIDCProviderARN reuses an account's existing GitHub OIDC provider; AWS allows
	// one per account, so set it if another stack or project already created one.
	GitHubOIDCProviderARN string
	// Env is the storefront's plain environment, Secrets its secret environment.
	Env     map[string]string
	Secrets map[string]pulumi.StringOutput
}

// Storefront environment variables, the same ones Oxygen injects.
var storefrontPlainEnv = []string{
	"PUBLIC_STORE_DOMAIN",
	"PUBLIC_STOREFRONT_ID",
	"PUBLIC_STOREFRONT_API_TOKEN",
	"PUBLIC_CHECKOUT_DOMAIN",
	"PUBLIC_CUSTOMER_ACCOUNT_API_CLIENT_ID",
	"PUBLIC_CUSTOMER_ACCOUNT_API_URL",
	"SHOP_ID",
	"BAKEHOUSE_API_URL",
	"BAKEHOUSE_KEY_ID",
}

var storefrontSecretEnv = []string{
	"PRIVATE_STOREFRONT_API_TOKEN",
	"BAKEHOUSE_HMAC_SECRET",
}

var requiredSecrets = []string{
	"SHOPIFY_API_SECRET",
	"STOREFRONT_HMAC_SECRETS",
	"SLOT_SIGNING_SECRET",
}

var optionalSecrets = []string{
	"RESEND_API_KEY",
	"TWILIO_ACCOUNT_SID",
	"TWILIO_AUTH_TOKEN",
	"TWILIO_MESSAGING_SERVICE_SID",
}

var ciSecretNames = []string{
	"SHOPIFY_HYDROGEN_DEPLOYMENT_TOKEN",
	"SHOPIFY_CLI_PARTNERS_TOKEN",
}

// Load reads and validates the current stack's configuration.
func Load(ctx *pulumi.Context) (*Config, error) {
	c := pcfg.New(ctx, "")
	stack := ctx.Stack()
	if stack != "staging" && stack != "prod" {
		return nil, fmt.Errorf("unsupported stack %q: use staging or prod", stack)
	}

	domain := c.Get("domain")
	cfg := &Config{
		Stack:            stack,
		IsProd:           stack == "prod",
		Domain:           domain,
		HasRender:        domain != "",
		ResendDKIM:       c.Get("resendDkim"),
		GitHubRepository: c.Get("githubRepository"),
		BakehouseSecrets: map[string]pulumi.StringOutput{},
		CISecrets:        map[string]pulumi.StringOutput{},
	}

	if cfg.HasRender {
		cfg.Region = orDefault(c.Get("region"), "oregon")
		cfg.BakehouseImage = orDefault(c.Get("bakehouseImage"), "ghcr.io/jcuffe/baked-blessings-bakehouse")
		cfg.BakehouseImageTag = orDefault(c.Get("bakehouseImageTag"), "latest")
		cfg.BakehousePlan = orDefault(c.Get("bakehousePlan"), "starter")
		cfg.PostgresPlan = orDefault(c.Get("postgresPlan"), "basic_256mb")
		cfg.PostgresVersion = orDefault(c.Get("postgresVersion"), "17")
		cfg.GhcrUsername = c.Get("ghcrUsername")
		cfg.GhcrToken = c.GetSecret("ghcrToken")
		cfg.ShopifyAPIKey = c.Require("SHOPIFY_API_KEY")
		cfg.ShopDomain = c.Require("SHOP_DOMAIN")
		cfg.Scopes = c.Require("SCOPES")
		for _, name := range requiredSecrets {
			cfg.BakehouseSecrets[name] = c.RequireSecret(name)
		}
		for _, name := range optionalSecrets {
			if c.Get(name) != "" {
				cfg.BakehouseSecrets[name] = c.RequireSecret(name)
			}
		}
		if c.Get("forwardTo") != "" {
			cfg.ForwardTo = c.RequireSecret("forwardTo")
			cfg.HasForward = true
		}
	}

	for _, name := range ciSecretNames {
		if c.Get(name) != "" {
			cfg.CISecrets[name] = c.RequireSecret(name)
		}
	}

	if region := pcfg.New(ctx, "aws").Get("region"); region != "" {
		sf := Storefront{
			Enabled:               true,
			Region:                region,
			ImageTag:              c.Get("storefrontImageTag"),
			MemoryMB:              orDefaultInt(c.GetInt("storefrontMemoryMb"), 1024),
			GitHubOIDCProviderARN: c.Get("githubOidcProviderArn"),
			Env:                   map[string]string{},
			Secrets:               map[string]pulumi.StringOutput{},
		}
		if sf.ImageTag != "" {
			sf.Env["PUBLIC_STORE_DOMAIN"] = c.Require("PUBLIC_STORE_DOMAIN")
			sf.Secrets["SESSION_SECRET"] = c.RequireSecret("SESSION_SECRET")
		}
		for _, name := range storefrontPlainEnv {
			if v := c.Get(name); v != "" {
				sf.Env[name] = v
			}
		}
		for _, name := range storefrontSecretEnv {
			if c.Get(name) != "" {
				sf.Secrets[name] = c.RequireSecret(name)
			}
		}
		cfg.Storefront = sf
	}

	return cfg, nil
}

// BakehouseHost is the public hostname of the Bakehouse app for this stack.
func (c *Config) BakehouseHost() string {
	if c.IsProd {
		return "bakehouse." + c.Domain
	}
	return "bakehouse.staging." + c.Domain
}

// ResourceName prefixes a provider-side resource name with the stack.
func (c *Config) ResourceName(base string) string {
	return base + "-" + c.Stack
}

func orDefaultInt(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
