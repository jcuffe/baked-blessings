// Package db provisions Bakehouse's Postgres database on Render.
package db

import (
	"github.com/pulumi/pulumi-terraform-provider/sdks/go/render/render"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/jcuffe/baked-blessings/infra/internal/config"
)

// Database is the provisioned Postgres instance.
type Database struct {
	// InternalURL is the private-network connection string Bakehouse uses.
	InternalURL pulumi.StringOutput
}

// New creates the Bakehouse Postgres database.
func New(ctx *pulumi.Context, cfg *config.Config) (*Database, error) {
	pg, err := render.NewPostgres(ctx, "bakehouse-db", &render.PostgresArgs{
		Name:         pulumi.String(cfg.ResourceName("bakehouse-db")),
		Plan:         pulumi.String(cfg.PostgresPlan),
		Region:       pulumi.String(cfg.Region),
		Version:      pulumi.String(cfg.PostgresVersion),
		DatabaseName: pulumi.String("bakehouse"),
		DatabaseUser: pulumi.String("bakehouse"),
	}, pulumi.Protect(cfg.IsProd))
	if err != nil {
		return nil, err
	}

	internalURL := pg.ConnectionInfo.InternalConnectionString().Elem()
	return &Database{InternalURL: pulumi.ToSecret(internalURL).(pulumi.StringOutput)}, nil
}
