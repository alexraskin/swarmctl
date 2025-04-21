package server

import (
	"context"
	"fmt"

	"github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/dns"
	"github.com/cloudflare/cloudflare-go/v4/option"
	"github.com/cloudflare/cloudflare-go/v4/zero_trust"
	"github.com/cloudflare/cloudflare-go/v4/zones"
	"golang.org/x/net/publicsuffix"
)

type CloudflareClient struct {
	client *cloudflare.Client
	config *Config
}

func NewCloudflareClient(config *Config) (*CloudflareClient, error) {
	client := cloudflare.NewClient(
		option.WithAPIKey(config.CloudflareAPIKey),
		option.WithAPIEmail(config.CloudflareAPIEmail),
	)
	return &CloudflareClient{client: client, config: config}, nil
}

func (c *CloudflareClient) updateTunnelConfig(ctx context.Context, hostname, serviceURL string) error {
	params := zero_trust.TunnelCloudflaredConfigurationUpdateParams{
		AccountID: cloudflare.F(c.config.CloudflareAccountID),

		Config: cloudflare.F(zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfig{
			Ingress: cloudflare.F([]zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
				{
					Hostname: cloudflare.F(hostname),
					Service:  cloudflare.F(serviceURL),
				},
				{
					Service: cloudflare.F("http_status:404"),
				},
			}),
			OriginRequest: cloudflare.F(zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigOriginRequest{
				Access: cloudflare.F(zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigOriginRequestAccess{}),
			}),
		}),
	}

	_, err := c.client.
		ZeroTrust.
		Tunnels.
		Cloudflared.
		Configurations.
		Update(ctx, c.config.CloudflareTunnelID, params)

	return err

}

func (c *CloudflareClient) getTunnelConfig(ctx context.Context) (*zero_trust.TunnelCloudflaredConfigurationGetResponse, error) {
	config, err := c.client.ZeroTrust.Tunnels.Cloudflared.Configurations.Get(ctx, c.config.CloudflareTunnelID, zero_trust.TunnelCloudflaredConfigurationGetParams{
		AccountID: cloudflare.F(c.config.CloudflareAccountID),
	})
	if err != nil {
		return nil, err
	}
	return config, nil
}

func (c *CloudflareClient) createTunnelDNSRecord(ctx context.Context, zoneID string, hostname string) error {
	recordParam := dns.CNAMERecordParam{
		Name:    cloudflare.F(hostname),
		Content: cloudflare.F(c.config.CloudflareTunnelID + ".cfargotunnel.com"),
		TTL:     cloudflare.F(dns.TTL(1)),
		Proxied: cloudflare.F(true),
		Type:    cloudflare.F(dns.CNAMERecordTypeCNAME),
	}

	params := dns.RecordNewParams{
		ZoneID: cloudflare.F(zoneID),
		Record: recordParam,
	}

	_, err := c.client.DNS.Records.New(ctx, params)
	return err
}

func (c *CloudflareClient) getZoneID(ctx context.Context, hostname string) (string, error) {
	domain, err := publicsuffix.EffectiveTLDPlusOne(hostname)
	if err != nil {
		return "", fmt.Errorf("cannot determine root domain for %q: %w", hostname, err)
	}

	resp, err := c.client.Zones.List(ctx, zones.ZoneListParams{
		Account: cloudflare.F(zones.ZoneListParamsAccount{ID: cloudflare.F(c.config.CloudflareAccountID)}),
		Name:    cloudflare.F(domain),
	})
	if err != nil {
		return "", fmt.Errorf("listing zones for %q: %w", domain, err)
	}
	if len(resp.Result) == 0 {
		return "", fmt.Errorf("no zone found for %q", domain)
	}
	return resp.Result[0].ID, nil
}
