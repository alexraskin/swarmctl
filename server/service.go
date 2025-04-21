package server

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/docker/docker/api/types/swarm"
)

type existingConfig struct {
	ID  string
	URL string
}

func (s *Server) StartCloudflare() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	slog.Debug("starting Cloudflare sync")

	if err := s.cloudflareSync(); err != nil {
		slog.Error("failed to sync Cloudflare hostnames", slog.Any("err", err))
	}

	for {
		select {
		case <-ticker.C:
			if err := s.cloudflareSync(); err != nil {
				slog.Error("failed to sync Cloudflare hostnames", slog.Any("err", err))
			}
		case <-s.ctx.Done():
			slog.Info("Stopping Cloudflare sync ticker")
			return
		}
	}
}

func (s *Server) cloudflareSync() error {
	existingConfigs, err := s.getExistingTunnelConfigs()
	if err != nil {
		return err
	}

	services, err := s.getDockerServices()
	if err != nil {
		return fmt.Errorf("get docker services: %w", err)
	}

	for _, service := range services {
		if err := s.processService(service, existingConfigs); err != nil {
			slog.Error("failed to process service", slog.String("service", service.Spec.Name), slog.Any("err", err))
		}
	}

	return nil
}

func (s *Server) getExistingTunnelConfigs() (map[string]existingConfig, error) {
	existingCfgs, err := s.cloudflareClient.getTunnelConfig(s.ctx)
	if err != nil {
		return nil, fmt.Errorf("list existing tunnel configs: %w", err)
	}

	existing := make(map[string]existingConfig)
	for _, cfg := range existingCfgs.Config.Ingress {
		existing[cfg.Hostname] = existingConfig{
			ID:  cfg.Hostname,
			URL: cfg.Service,
		}
	}

	return existing, nil
}

func (s *Server) processService(service swarm.Service, existingConfigs map[string]existingConfig) error {
	serviceName := service.Spec.Name

	host, port, err := s.getDockerServiceMetadata(serviceName)
	if err != nil {
		slog.Debug("skipping service", slog.String("service", serviceName), slog.String("reason", err.Error()))
		return nil
	}

	parts := strings.Split(service.Spec.Name, "_")
	internalName := parts[len(parts)-1]
	internalServiceURL := fmt.Sprintf("http://%s:%s", internalName, port)

	if _, exists := existingConfigs[host]; !exists {
		err := s.cloudflareClient.updateTunnelConfig(s.ctx, host, internalServiceURL)
		if err != nil {
			return fmt.Errorf("failed to create hostname %s: %w", host, err)
		}
		slog.Debug("created new tunnel rule", slog.String("hostname", host), slog.String("service", internalServiceURL))

		zoneID, err := s.cloudflareClient.getZoneID(s.ctx, host)
		if err != nil {
			return fmt.Errorf("failed to get zone ID for hostname %s: %w", host, err)
		}

		slog.Debug("got zone ID", slog.String("zoneID", zoneID))

		err = s.cloudflareClient.createTunnelDNSRecord(s.ctx, zoneID, host)
		if err != nil {
			return fmt.Errorf("failed to create DNS record for hostname %s: %w", host, err)
		}

		slog.Debug("created DNS record", slog.String("hostname", host), slog.String("zoneID", zoneID))
	} else {
		slog.Debug("tunnel hostname already exists", slog.String("hostname", host))
	}

	return nil
}
