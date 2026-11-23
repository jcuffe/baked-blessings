// Infrastructure for Jakki's Baked Blessings. See README.md and docs/SPEC.md.
package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/jcuffe/baked-blessings/infra/internal/app"
	"github.com/jcuffe/baked-blessings/infra/internal/ci"
	"github.com/jcuffe/baked-blessings/infra/internal/config"
	"github.com/jcuffe/baked-blessings/infra/internal/db"
	"github.com/jcuffe/baked-blessings/infra/internal/dns"
	"github.com/jcuffe/baked-blessings/infra/internal/storefront"
)

func main() {
	pulumi.Run(run)
}

func run(ctx *pulumi.Context) error {
	cfg, err := config.Load(ctx)
	if err != nil {
		return err
	}

	ciVariables := map[string]pulumi.StringOutput{}

	if cfg.Storefront.Enabled {
		sf, err := storefront.New(ctx, cfg)
		if err != nil {
			return err
		}
		ctx.Export("storefrontUrl", sf.URL)
		ctx.Export("storefrontRepositoryUrl", sf.RepositoryURL)
		ctx.Export("storefrontFunctionName", sf.FunctionName)
		ctx.Export("storefrontCiRoleArn", sf.CIRoleARN)
		ciVariables["AWS_REGION"] = pulumi.String(cfg.Storefront.Region).ToStringOutput()
		ciVariables["AWS_ROLE_ARN"] = sf.CIRoleARN
		ciVariables["STOREFRONT_ECR_REPOSITORY_URL"] = sf.RepositoryURL
		ciVariables["STOREFRONT_LAMBDA_FUNCTION"] = sf.FunctionName
	}

	// Bakehouse and Postgres on Render with Cloudflare DNS, until they move to AWS.
	if cfg.HasRender {
		zone, err := dns.LookupZone(ctx, cfg.Domain)
		if err != nil {
			return err
		}
		database, err := db.New(ctx, cfg)
		if err != nil {
			return err
		}
		bakehouse, err := app.New(ctx, cfg, database)
		if err != nil {
			return err
		}
		if err := dns.Records(ctx, cfg, zone, bakehouse.OnRenderHost); err != nil {
			return err
		}
		ctx.Export("bakehouseUrl", pulumi.String("https://"+cfg.BakehouseHost()))
		ctx.Export("bakehouseOnRender", bakehouse.OnRenderHost)
	}

	return ci.Environment(ctx, cfg, ciVariables)
}
