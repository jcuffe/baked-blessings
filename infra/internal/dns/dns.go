// Package dns manages the Cloudflare zone: Shopify storefront, Bakehouse, email sending
// (Resend) and inbound forwarding (Cloudflare Email Routing).
package dns

import (
	"github.com/pulumi/pulumi-cloudflare/sdk/v6/go/cloudflare"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/jcuffe/baked-blessings/infra/internal/config"
)

// Shopify's documented targets for connecting a custom domain (storefront on Oxygen).
const (
	shopifyApexA    = "23.227.38.65"
	shopifyApexAAAA = "2620:0127:f00f:5::"
	shopifyWWW      = "shops.myshopify.com"
)

// Zone is the looked-up Cloudflare zone for the bakery's domain.
type Zone struct {
	ID        string
	AccountID string
}

// LookupZone finds the existing zone. The domain itself is purchased by hand.
func LookupZone(ctx *pulumi.Context, domain string) (*Zone, error) {
	z, err := cloudflare.LookupZone(ctx, &cloudflare.LookupZoneArgs{
		Filter: &cloudflare.GetZoneFilter{Name: pulumi.StringRef(domain), Match: "all"},
	})
	if err != nil {
		return nil, err
	}
	return &Zone{ID: z.Id, AccountID: z.Account.Id}, nil
}

// Records creates the stack's DNS records. Domain-wide records (storefront, email) are
// only managed by the prod stack; staging only adds its own Bakehouse hostname.
func Records(ctx *pulumi.Context, cfg *config.Config, zone *Zone, bakehouseTarget pulumi.StringInput) error {
	r := recorder{ctx: ctx, zoneID: zone.ID}

	r.add("bakehouse", cfg.BakehouseHost(), "CNAME", bakehouseTarget, nil)

	if !cfg.IsProd {
		return r.err
	}

	// Storefront (Shopify connects the domain to the Hydrogen storefront).
	r.add("shopify-apex-a", cfg.Domain, "A", pulumi.String(shopifyApexA), nil)
	r.add("shopify-apex-aaaa", cfg.Domain, "AAAA", pulumi.String(shopifyApexAAAA), nil)
	r.add("shopify-www", "www."+cfg.Domain, "CNAME", pulumi.String(shopifyWWW), nil)

	// Outbound mail via Resend on the "send" subdomain; values from the Resend dashboard.
	if cfg.ResendDKIM != "" {
		r.add("resend-mx", "send."+cfg.Domain, "MX", pulumi.String("feedback-smtp.us-east-1.amazonses.com"), pulumi.Float64(10))
		r.add("resend-spf", "send."+cfg.Domain, "TXT", pulumi.String("\"v=spf1 include:amazonses.com ~all\""), nil)
		r.add("resend-dkim", "resend._domainkey."+cfg.Domain, "TXT", pulumi.String(cfg.ResendDKIM), nil)
	}
	r.add("dmarc", "_dmarc."+cfg.Domain, "TXT", pulumi.String("\"v=DMARC1; p=none;\""), nil)

	if r.err != nil {
		return r.err
	}
	if cfg.HasForward {
		return emailRouting(ctx, cfg, zone)
	}
	return nil
}

// emailRouting forwards hello@<domain> to Jakki's personal inbox. Cloudflare adds its own
// MX/SPF/DKIM records when routing is enabled; the destination address must be verified
// once by clicking the link Cloudflare emails to it.
func emailRouting(ctx *pulumi.Context, cfg *config.Config, zone *Zone) error {
	if _, err := cloudflare.NewEmailRoutingDns(ctx, "email-routing", &cloudflare.EmailRoutingDnsArgs{
		ZoneId: pulumi.String(zone.ID),
	}); err != nil {
		return err
	}
	if _, err := cloudflare.NewEmailRoutingAddress(ctx, "forward-to", &cloudflare.EmailRoutingAddressArgs{
		AccountId: pulumi.String(zone.AccountID),
		Email:     cfg.ForwardTo,
	}); err != nil {
		return err
	}
	_, err := cloudflare.NewEmailRoutingRule(ctx, "hello", &cloudflare.EmailRoutingRuleArgs{
		ZoneId:  pulumi.String(zone.ID),
		Name:    pulumi.String("hello@ to Jakki"),
		Enabled: pulumi.Bool(true),
		Matchers: cloudflare.EmailRoutingRuleMatcherArray{
			cloudflare.EmailRoutingRuleMatcherArgs{
				Type:  pulumi.String("literal"),
				Field: pulumi.String("to"),
				Value: pulumi.String("hello@" + cfg.Domain),
			},
		},
		Actions: cloudflare.EmailRoutingRuleActionArray{
			cloudflare.EmailRoutingRuleActionArgs{
				Type:   pulumi.String("forward"),
				Values: pulumi.StringArray{cfg.ForwardTo},
			},
		},
	})
	return err
}

// recorder creates DNS-only (unproxied) records and keeps the first error.
type recorder struct {
	ctx    *pulumi.Context
	zoneID string
	err    error
}

func (r *recorder) add(name, host, typ string, content pulumi.StringInput, priority pulumi.Float64PtrInput) {
	if r.err != nil {
		return
	}
	_, r.err = cloudflare.NewDnsRecord(r.ctx, name, &cloudflare.DnsRecordArgs{
		ZoneId:   pulumi.String(r.zoneID),
		Name:     pulumi.String(host),
		Type:     pulumi.String(typ),
		Content:  content,
		Priority: priority,
		Proxied:  pulumi.Bool(false),
		Ttl:      pulumi.Float64(1), // automatic
	})
}
