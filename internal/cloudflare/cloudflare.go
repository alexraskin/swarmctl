package cloudflare

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/dns"
	"github.com/cloudflare/cloudflare-go/v4/option"
	"github.com/cloudflare/cloudflare-go/v4/zero_trust"
	"github.com/cloudflare/cloudflare-go/v4/zones"
	"golang.org/x/net/publicsuffix"
)

type CloudflareClient struct {
	client              *cloudflare.Client
	apiKey              string
	apiEmail            string
	cloudflareTunnelID  string
	cloudflareAccountID string
}

func NewCloudflareClient(apiKey string, apiEmail string, cloudflareTunnelID string, cloudflareAccountID string) (*CloudflareClient, error) {
	client := cloudflare.NewClient(
		option.WithAPIKey(apiKey),
		option.WithAPIEmail(apiEmail),
	)
	return &CloudflareClient{client: client, apiKey: apiKey, apiEmail: apiEmail, cloudflareTunnelID: cloudflareTunnelID, cloudflareAccountID: cloudflareAccountID}, nil
}

func (c *CloudflareClient) UpdateTunnelConfig(ctx context.Context, hostname, serviceURL string) error {

	existingConfig, err := c.GetTunnelConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get existing tunnel config: %w", err)
	}

	ingressList := []zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
		{
			Hostname: cloudflare.F(hostname),
			Service:  cloudflare.F(serviceURL),
		},
	}

	for _, ingress := range existingConfig.Config.Ingress {
		if ingress.Service == "http_status:404" {
			continue
		}

		if ingress.Hostname == hostname {
			continue
		}

		if strings.Contains(ingress.Hostname, ",") {
			continue
		}

		ingressList = append(ingressList, zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
			Hostname: cloudflare.F(ingress.Hostname),
			Service:  cloudflare.F(ingress.Service),
		})
	}

	ingressList = append(ingressList, zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
		Service: cloudflare.F("http_status:404"),
	})

	params := zero_trust.TunnelCloudflaredConfigurationUpdateParams{
		AccountID: cloudflare.F(c.cloudflareAccountID),
		Config: cloudflare.F(zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfig{
			Ingress: cloudflare.F(ingressList),
			OriginRequest: cloudflare.F(zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigOriginRequest{
				Access: cloudflare.F(zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigOriginRequestAccess{}),
			}),
		}),
	}

	_, err = c.client.
		ZeroTrust.
		Tunnels.
		Cloudflared.
		Configurations.
		Update(ctx, c.cloudflareTunnelID, params)

	return err
}

func (c *CloudflareClient) GetTunnelConfig(ctx context.Context) (*zero_trust.TunnelCloudflaredConfigurationGetResponse, error) {
	config, err := c.client.ZeroTrust.Tunnels.Cloudflared.Configurations.Get(ctx, c.cloudflareTunnelID, zero_trust.TunnelCloudflaredConfigurationGetParams{
		AccountID: cloudflare.F(c.cloudflareAccountID),
	})
	if err != nil {
		return nil, err
	}
	return config, nil
}

func (c *CloudflareClient) CreateTunnelDNSRecord(ctx context.Context, zoneID string, hostname string) error {
	recordParam := dns.CNAMERecordParam{
		Name:    cloudflare.F(hostname),
		Content: cloudflare.F(c.cloudflareTunnelID + ".cfargotunnel.com"),
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

func (c *CloudflareClient) GetZoneID(ctx context.Context, hostname string) (string, error) {
	domain, err := publicsuffix.EffectiveTLDPlusOne(hostname)
	if err != nil {
		return "", fmt.Errorf("cannot determine root domain for %q: %w", hostname, err)
	}

	resp, err := c.client.Zones.List(ctx, zones.ZoneListParams{
		Account: cloudflare.F(zones.ZoneListParamsAccount{ID: cloudflare.F(c.cloudflareAccountID)}),
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
