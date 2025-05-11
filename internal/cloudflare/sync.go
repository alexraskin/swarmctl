package cloudflare

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudflare/cloudflare-go/v4/zero_trust"
	"github.com/docker/docker/api/types/swarm"
)

type API interface {
	GetTunnelConfig(ctx context.Context) (*zero_trust.TunnelCloudflaredConfigurationGetResponse, error)
	UpdateTunnelConfig(ctx context.Context, hostname, targetURL string) error
	GetZoneID(ctx context.Context, hostname string) (string, error)
	CreateTunnelDNSRecord(ctx context.Context, zoneID, hostname string) error
	DeleteTunnelDNSRecord(ctx context.Context, recordID string, zoneID string) error
	GetTunnelDNSRecord(ctx context.Context, zoneID string, hostname string) (string, error)
}

type existingConfig struct {
	ID  string
	URL string
}

type Syncer struct {
	client API
	mu     sync.Mutex
	cache  map[string]existingConfig
}

func NewSyncer(client API) *Syncer {
	return &Syncer{client: client}
}

func (s *Syncer) LoadExisting(ctx context.Context) (map[string]existingConfig, error) {
	resp, err := s.client.GetTunnelConfig(ctx)
	if err != nil {
		return nil, err
	}

	cache := make(map[string]existingConfig)
	for _, entry := range resp.Config.Ingress {
		cache[entry.Hostname] = existingConfig{ID: entry.Hostname, URL: entry.Service}
	}
	return cache, nil
}

func (s *Syncer) SyncService(ctx context.Context, svc *swarm.Service) error {
	s.mu.Lock()
	if s.cache == nil {
		s.cache = make(map[string]existingConfig)
		rep, err := s.client.GetTunnelConfig(ctx)
		if err != nil {
			s.mu.Unlock()
			return fmt.Errorf("initial load: %w", err)
		}
		for _, e := range rep.Config.Ingress {
			s.cache[e.Hostname] = existingConfig{ID: e.Hostname, URL: e.Service}
		}
	}
	s.mu.Unlock()

	labels := svc.Spec.Labels
	target := fmt.Sprintf("http://%s:%s", svc.Spec.Name, labels["cloudflared.tunnel.port"])

	hosts := []string{labels["cloudflared.tunnel.hostname"]}
	for k, v := range labels {
		if strings.HasSuffix(k, ".hostname") && v != "" {
			hosts = append(hosts, v)
		}
	}

	for _, list := range hosts {
		for h0 := range strings.SplitSeq(list, ",") {
			h := strings.TrimSpace(h0)
			if h == "" {
				continue
			}

			ex, ok := s.cache[h]
			if !ok || ex.URL != target {
				if err := s.client.UpdateTunnelConfig(ctx, h, target); err != nil {
					return fmt.Errorf("update %s: %w", h, err)
				}
			}

			if !ok {
				zoneID, err := s.client.GetZoneID(ctx, h)
				if err != nil {
					return fmt.Errorf("zone %s: %w", h, err)
				}
				if err := s.client.CreateTunnelDNSRecord(ctx, zoneID, h); err != nil {
					return fmt.Errorf("dns %s: %w", h, err)
				}
			}

			s.mu.Lock()
			s.cache[h] = existingConfig{ID: h, URL: target}
			s.mu.Unlock()
		}
	}
	return nil
}
