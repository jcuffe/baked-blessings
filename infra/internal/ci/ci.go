// Package ci manages the GitHub Actions environment and deploy secrets for each stack.
package ci

import (
	"strings"

	"github.com/pulumi/pulumi-github/sdk/v6/go/github"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/jcuffe/baked-blessings/infra/internal/config"
)

// Environment creates the stack's Actions environment and its secrets. Bootstrap secrets
// (PULUMI_ACCESS_TOKEN, provider tokens) are set by hand because CI needs them to run Pulumi.
func Environment(ctx *pulumi.Context, cfg *config.Config) error {
	if cfg.GitHubRepository == "" {
		return nil
	}
	repo := cfg.GitHubRepository
	if i := strings.Index(repo, "/"); i >= 0 {
		repo = repo[i+1:] // the provider's owner comes from github:owner
	}

	env, err := github.NewRepositoryEnvironment(ctx, "actions-env", &github.RepositoryEnvironmentArgs{
		Repository:  pulumi.String(repo),
		Environment: pulumi.String(cfg.Stack),
	})
	if err != nil {
		return err
	}

	for name, value := range cfg.CISecrets {
		if _, err := github.NewActionsEnvironmentSecret(ctx, "ci-"+strings.ToLower(name), &github.ActionsEnvironmentSecretArgs{
			Repository:  pulumi.String(repo),
			Environment: env.Environment,
			SecretName:  pulumi.String(name),
			Value:       value,
		}); err != nil {
			return err
		}
	}
	return nil
}
