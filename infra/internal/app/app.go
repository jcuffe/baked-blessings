// Package app provisions the Bakehouse web service (admin UI, storefront API, webhooks and
// the inline worker) on Render.
package app

import (
	"strings"

	"github.com/pulumi/pulumi-terraform-provider/sdks/go/render/render"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/jcuffe/baked-blessings/infra/internal/config"
	"github.com/jcuffe/baked-blessings/infra/internal/db"
)

// Bakehouse is the deployed web service.
type Bakehouse struct {
	// OnRenderHost is the service's *.onrender.com hostname, the CNAME target for BakehouseHost.
	OnRenderHost pulumi.StringOutput
}

// New creates the Bakehouse web service from the container image in GHCR.
func New(ctx *pulumi.Context, cfg *config.Config, database *db.Database) (*Bakehouse, error) {
	env := render.WebServiceEnvVarsMap{
		"NODE_ENV":        plain("production"),
		"WORKER_MODE":     plain("inline"),
		"SHOPIFY_APP_URL": plain("https://" + cfg.BakehouseHost()),
		"SHOPIFY_API_KEY": plain(cfg.ShopifyAPIKey),
		"SHOP_DOMAIN":     plain(cfg.ShopDomain),
		"SCOPES":          plain(cfg.Scopes),
		"DATABASE_URL":    render.WebServiceEnvVarsArgs{Value: database.InternalURL},
	}
	for name, value := range cfg.BakehouseSecrets {
		env[name] = render.WebServiceEnvVarsArgs{Value: value}
	}

	image := &render.WebServiceRuntimeSourceImageArgs{
		ImageUrl: pulumi.String(cfg.BakehouseImage),
		Tag:      pulumi.String(cfg.BakehouseImageTag),
	}
	if cfg.GhcrUsername != "" {
		cred, err := render.NewRegistryCredential(ctx, "ghcr", &render.RegistryCredentialArgs{
			Name:      pulumi.String(cfg.ResourceName("ghcr")),
			Registry:  pulumi.String("GITHUB"),
			Username:  pulumi.String(cfg.GhcrUsername),
			AuthToken: cfg.GhcrToken,
		})
		if err != nil {
			return nil, err
		}
		image.RegistryCredentialId = cred.ID().ToStringOutput()
	}

	svc, err := render.NewWebService(ctx, "bakehouse", &render.WebServiceArgs{
		Name:            pulumi.String(cfg.ResourceName("bakehouse")),
		Plan:            pulumi.String(cfg.BakehousePlan),
		Region:          pulumi.String(cfg.Region),
		HealthCheckPath: pulumi.String("/healthz"),
		RuntimeSource:   render.WebServiceRuntimeSourceArgs{Image: image},
		EnvVars:         env,
		CustomDomains: render.WebServiceCustomDomainArray{
			render.WebServiceCustomDomainArgs{Name: pulumi.String(cfg.BakehouseHost())},
		},
	})
	if err != nil {
		return nil, err
	}

	host := svc.Url.ApplyT(func(u string) string {
		return strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	}).(pulumi.StringOutput)

	return &Bakehouse{OnRenderHost: host}, nil
}

func plain(v string) render.WebServiceEnvVarsArgs {
	return render.WebServiceEnvVarsArgs{Value: pulumi.String(v)}
}
