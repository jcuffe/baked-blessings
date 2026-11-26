package storefront

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/jcuffe/baked-blessings/infra/internal/config"
)

type mocks struct {
	mu        sync.Mutex
	resources map[string]resource.PropertyMap // "type::name" -> inputs
}

func (m *mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resources[args.TypeToken+"::"+args.Name] = args.Inputs
	outs := args.Inputs.Copy()
	outs["arn"] = resource.NewStringProperty("arn:aws:mock:::" + args.Name)
	switch args.TypeToken {
	case "aws:ecr/repository:Repository":
		outs["repositoryUrl"] = resource.NewStringProperty("123.dkr.ecr.us-east-1.amazonaws.com/" + args.Inputs["name"].StringValue())
	case "aws:lambda/functionUrl:FunctionUrl":
		outs["functionUrl"] = resource.NewStringProperty("https://abc.lambda-url.us-east-1.on.aws/")
	case "aws:cloudfront/distribution:Distribution":
		outs["domainName"] = resource.NewStringProperty("d123.cloudfront.net")
	case "random:index/randomPassword:RandomPassword":
		outs["result"] = resource.MakeSecret(resource.NewStringProperty("secret"))
	}
	return args.Name + "-id", outs, nil
}

func (m *mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

func run(t *testing.T, cfg *config.Config) (*mocks, map[string]string) {
	t.Helper()
	m := &mocks{resources: map[string]resource.PropertyMap{}}
	got := map[string]string{}
	var mu sync.Mutex
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		sf, err := New(ctx, cfg)
		if err != nil {
			return err
		}
		for name, out := range map[string]pulumi.StringOutput{"url": sf.URL, "function": sf.FunctionName, "repo": sf.RepositoryURL} {
			out.ApplyT(func(v string) string {
				mu.Lock()
				defer mu.Unlock()
				got[name] = v
				return v
			})
		}
		return nil
	}, pulumi.WithMocks("baked-blessings", cfg.Stack, m))
	if err != nil {
		t.Fatal(err)
	}
	return m, got
}

func names(m *mocks) []string {
	var out []string
	for k := range m.resources {
		out = append(out, k[strings.LastIndex(k, "::")+2:])
	}
	slices.Sort(out)
	return out
}

func prodConfig(tag string) *config.Config {
	return &config.Config{
		Stack:            "prod",
		IsProd:           true,
		GitHubRepository: "jcuffe/baked-blessings",
		Storefront: config.Storefront{
			Enabled:  true,
			Region:   "us-east-1",
			ImageTag: tag,
			MemoryMB: 1024,
			Env:      map[string]string{"PUBLIC_STORE_DOMAIN": "example.myshopify.com"},
		},
	}
}

func TestBootstrapCreatesOnlyRepositoryAndCIRole(t *testing.T) {
	m, got := run(t, prodConfig(""))
	want := []string{"github-actions", "storefront", "storefront", "storefront-ci", "storefront-ci"}
	if !slices.Equal(names(m), want) {
		t.Fatalf("resources = %v, want %v", names(m), want)
	}
	if got["url"] != "" || got["function"] != "" {
		t.Fatalf("bootstrap should not serve anything yet: %v", got)
	}
}

func TestImageTagServesThroughCloudFront(t *testing.T) {
	m, got := run(t, prodConfig("abc123"))
	for _, key := range []string{
		"aws:lambda/function:Function::storefront",
		"aws:lambda/functionUrl:FunctionUrl::storefront",
		"aws:cloudfront/distribution:Distribution::storefront",
	} {
		if _, ok := m.resources[key]; !ok {
			t.Errorf("missing %s", key)
		}
	}
	fn := m.resources["aws:lambda/function:Function::storefront"]
	if uri := fn["imageUri"].StringValue(); !strings.HasSuffix(uri, ":abc123") {
		t.Errorf("imageUri = %q", uri)
	}
	if arch := fn["architectures"].ArrayValue()[0].StringValue(); arch != "arm64" {
		t.Errorf("architecture = %q, want arm64 to match the CI build", arch)
	}
	url := m.resources["aws:lambda/functionUrl:FunctionUrl::storefront"]
	if url["invokeMode"].StringValue() != "RESPONSE_STREAM" {
		t.Errorf("function URL should stream responses")
	}
	if got["url"] != "https://d123.cloudfront.net" {
		t.Errorf("url = %q", got["url"])
	}
}

func TestCIRoleIsScopedToTheStackEnvironment(t *testing.T) {
	m, _ := run(t, prodConfig(""))
	role := m.resources["aws:iam/role:Role::storefront-ci"]
	trust := role["assumeRolePolicy"].StringValue()
	if !strings.Contains(trust, `"repo:jcuffe/baked-blessings:environment:prod"`) {
		t.Fatalf("trust policy = %s", trust)
	}
}

func TestExistingOIDCProviderIsReused(t *testing.T) {
	cfg := prodConfig("")
	cfg.Storefront.GitHubOIDCProviderARN = "arn:aws:iam::123:oidc-provider/token.actions.githubusercontent.com"
	m, _ := run(t, cfg)
	if _, ok := m.resources["aws:iam/openIdConnectProvider:OpenIdConnectProvider::github-actions"]; ok {
		t.Fatal("should not create a second GitHub OIDC provider")
	}
}
