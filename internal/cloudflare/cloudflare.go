package cloudflare

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/dns"
	"github.com/cloudflare/cloudflare-go/v4/option"
	"github.com/cloudflare/cloudflare-go/v4/shared"
	"github.com/cloudflare/cloudflare-go/v4/zero_trust"
	"github.com/cloudflare/cloudflare-go/v4/zones"
	"golang.org/x/net/publicsuffix"
)

type CloudflareClient struct {
	client              *cloudflare.Client
	cloudflareTunnelID  string
	cloudflareAccountID string
	// configMu serializes the read-modify-write of the tunnel ingress config so
	// concurrent callers (service-event sync vs. removal reconciler) cannot lose
	// each other's updates.
	configMu sync.Mutex
}

// NewCloudflareClient authenticates with a scoped API token (Bearer), not the
// account-wide global key. The token needs Account: Cloudflare Tunnel: Edit,
// Account: Access: Apps and Policies: Edit, Zone: DNS: Edit, and Zone: Zone:
// Read.
func NewCloudflareClient(apiToken string, cloudflareTunnelID string, cloudflareAccountID string) (*CloudflareClient, error) {
	client := cloudflare.NewClient(
		option.WithAPIToken(apiToken),
	)
	return &CloudflareClient{
		client:              client,
		cloudflareTunnelID:  cloudflareTunnelID,
		cloudflareAccountID: cloudflareAccountID,
	}, nil
}

func (c *CloudflareClient) UpdateTunnelConfig(ctx context.Context, hostname, serviceURL string) error {
	c.configMu.Lock()
	defer c.configMu.Unlock()

	existingConfig, err := c.GetTunnelConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get existing tunnel config: %w", err)
	}

	ingressList := []zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{}

	// If serviceURL is not empty, add/update the hostname at the beginning
	if serviceURL != "" {
		ingressList = append(ingressList, zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
			Hostname: cloudflare.F(hostname),
			Service:  cloudflare.F(serviceURL),
		})
	}

	// Preserve all existing entries except:
	// - The 404 catch-all (we'll add it at the end)
	// - The hostname we're updating/removing (re-added above if serviceURL set)
	// Any other entry, including externally-managed comma-joined hostnames, is
	// carried through untouched.
	for _, ingress := range existingConfig.Config.Ingress {
		if ingress.Service == "http_status:404" {
			continue
		}

		if ingress.Hostname == hostname {
			continue
		}

		ingressList = append(ingressList, zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
			Hostname: cloudflare.F(ingress.Hostname),
			Service:  cloudflare.F(ingress.Service),
		})
	}

	// Always add the 404 catch-all at the end
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
	// Idempotent: skip creation if a CNAME for this hostname already exists, so a
	// re-created service does not produce duplicate records (which would later
	// break lookups in GetTunnelDNSRecord).
	existing, err := c.client.DNS.Records.List(ctx, dns.RecordListParams{
		ZoneID: cloudflare.F(zoneID),
		Name: cloudflare.F(dns.RecordListParamsName{
			Exact: cloudflare.F(hostname),
		}),
		Type: cloudflare.F(dns.RecordListParamsType(dns.CNAMERecordTypeCNAME)),
	})
	if err != nil {
		return fmt.Errorf("checking for existing DNS record %q: %w", hostname, err)
	}
	if len(existing.Result) > 0 {
		return nil
	}

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

	_, err = c.client.DNS.Records.New(ctx, params)
	return err
}

func (c *CloudflareClient) DeleteTunnelDNSRecord(ctx context.Context, recordID string, zoneID string) error {
	_, err := c.client.DNS.Records.Delete(ctx, recordID, dns.RecordDeleteParams{
		ZoneID: cloudflare.F(zoneID),
	})
	return err
}

func (c *CloudflareClient) GetTunnelDNSRecord(ctx context.Context, zoneID string, hostname string) (string, error) {
	record, err := c.client.DNS.Records.List(ctx, dns.RecordListParams{
		ZoneID: cloudflare.F(zoneID),
		Name: cloudflare.F(dns.RecordListParamsName{
			Contains: cloudflare.F(hostname),
		}),
		Type: cloudflare.F(dns.RecordListParamsType(dns.CNAMERecordTypeCNAME)),
	})
	if err != nil {
		return "", err
	}
	if len(record.Result) == 0 {
		return "", fmt.Errorf("no record found for %q", hostname)
	}
	if len(record.Result) > 1 {
		return "", fmt.Errorf("multiple records found for %q", hostname)
	}
	return record.Result[0].ID, nil
}

// findAccessApp returns the ID of the self-hosted Access application whose
// domain exactly matches hostname, if one exists.
func (c *CloudflareClient) findAccessApp(ctx context.Context, hostname string) (string, bool, error) {
	page, err := c.client.ZeroTrust.Access.Applications.List(ctx, zero_trust.AccessApplicationListParams{
		AccountID: cloudflare.F(c.cloudflareAccountID),
		Domain:    cloudflare.F(hostname),
	})
	if err != nil {
		return "", false, err
	}
	for _, app := range page.Result {
		if app.Domain == hostname {
			return app.ID, true, nil
		}
	}
	return "", false, nil
}

func (c *CloudflareClient) EnsureAccessApp(ctx context.Context, hostname, policyID string) error {
	_, found, err := c.findAccessApp(ctx, hostname)
	if err != nil {
		return fmt.Errorf("listing access apps for %q: %w", hostname, err)
	}
	if found {
		return nil
	}

	_, err = c.client.ZeroTrust.Access.Applications.New(ctx, zero_trust.AccessApplicationNewParams{
		AccountID: cloudflare.F(c.cloudflareAccountID),
		Body: zero_trust.AccessApplicationNewParamsBodySelfHostedApplication{
			Type:   cloudflare.F("self_hosted"),
			Name:   cloudflare.F(hostname),
			Domain: cloudflare.F(hostname),
			Policies: cloudflare.F([]zero_trust.AccessApplicationNewParamsBodySelfHostedApplicationPolicyUnion{
				shared.UnionString(policyID),
			}),
		},
	})
	if err != nil {
		return fmt.Errorf("creating access app for %q: %w", hostname, err)
	}
	return nil
}

func (c *CloudflareClient) DeleteAccessApp(ctx context.Context, hostname string) error {
	id, found, err := c.findAccessApp(ctx, hostname)
	if err != nil {
		return fmt.Errorf("listing access apps for %q: %w", hostname, err)
	}
	if !found {
		return nil
	}

	_, err = c.client.ZeroTrust.Access.Applications.Delete(ctx, id, zero_trust.AccessApplicationDeleteParams{
		AccountID: cloudflare.F(c.cloudflareAccountID),
	})
	if err != nil {
		return fmt.Errorf("deleting access app %q: %w", hostname, err)
	}
	return nil
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
