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
	Domain string
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

	cfg := &Config{
		Stack:             stack,
		IsProd:            stack == "prod",
		Domain:            c.Require("domain"),
		Region:            orDefault(c.Get("region"), "oregon"),
		BakehouseImage:    orDefault(c.Get("bakehouseImage"), "ghcr.io/jcuffe/baked-blessings-bakehouse"),
		BakehouseImageTag: orDefault(c.Get("bakehouseImageTag"), "latest"),
		BakehousePlan:     orDefault(c.Get("bakehousePlan"), "starter"),
		PostgresPlan:      orDefault(c.Get("postgresPlan"), "basic_256mb"),
		PostgresVersion:   orDefault(c.Get("postgresVersion"), "17"),
		GhcrUsername:      c.Get("ghcrUsername"),
		GhcrToken:         c.GetSecret("ghcrToken"),
		ShopifyAPIKey:     c.Require("SHOPIFY_API_KEY"),
		ShopDomain:        c.Require("SHOP_DOMAIN"),
		Scopes:            c.Require("SCOPES"),
		BakehouseSecrets:  map[string]pulumi.StringOutput{},
		ResendDKIM:        c.Get("resendDkim"),
		GitHubRepository:  c.Get("githubRepository"),
		CISecrets:         map[string]pulumi.StringOutput{},
	}

	for _, name := range requiredSecrets {
		cfg.BakehouseSecrets[name] = c.RequireSecret(name)
	}
	for _, name := range optionalSecrets {
		if c.Get(name) != "" {
			cfg.BakehouseSecrets[name] = c.RequireSecret(name)
		}
	}
	for _, name := range ciSecretNames {
		if c.Get(name) != "" {
			cfg.CISecrets[name] = c.RequireSecret(name)
		}
	}
	if c.Get("forwardTo") != "" {
		cfg.ForwardTo = c.RequireSecret("forwardTo")
		cfg.HasForward = true
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

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
