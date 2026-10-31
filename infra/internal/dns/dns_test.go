package dns

import (
	"slices"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/jcuffe/baked-blessings/infra/internal/config"
)

type mocks struct {
	mu    sync.Mutex
	names []string
}

func (m *mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.names = append(m.names, args.Name)
	return args.Name + "-id", args.Inputs, nil
}

func (m *mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

func recordsFor(t *testing.T, cfg *config.Config) []string {
	t.Helper()
	m := &mocks{}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		return Records(ctx, cfg, &Zone{ID: "zone", AccountID: "acct"}, pulumi.String("bakehouse.onrender.com"))
	}, pulumi.WithMocks("baked-blessings", cfg.Stack, m))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(m.names)
	return m.names
}

func TestStagingOnlyManagesItsOwnHost(t *testing.T) {
	got := recordsFor(t, &config.Config{Stack: "staging", Domain: "example.com", ResendDKIM: "dkim"})
	if !slices.Equal(got, []string{"bakehouse"}) {
		t.Fatalf("staging records = %v, want only bakehouse", got)
	}
}

func TestProdManagesStorefrontAndEmail(t *testing.T) {
	got := recordsFor(t, &config.Config{Stack: "prod", IsProd: true, Domain: "example.com", ResendDKIM: "dkim"})
	want := []string{
		"bakehouse", "dmarc", "resend-dkim", "resend-mx", "resend-spf",
		"shopify-apex-a", "shopify-apex-aaaa", "shopify-www",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("prod records = %v, want %v", got, want)
	}
}

func TestBakehouseHostPerStack(t *testing.T) {
	prod := &config.Config{Stack: "prod", IsProd: true, Domain: "example.com"}
	staging := &config.Config{Stack: "staging", Domain: "example.com"}
	if prod.BakehouseHost() != "bakehouse.example.com" || staging.BakehouseHost() != "bakehouse.staging.example.com" {
		t.Fatalf("hosts = %q, %q", prod.BakehouseHost(), staging.BakehouseHost())
	}
}
