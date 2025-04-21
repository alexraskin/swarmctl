package server

import (
	"fmt"
	"log/slog"
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
		s.logger.Error("failed to sync Cloudflare hostnames", slog.Any("err", err))
	}

	for {
		select {
		case <-ticker.C:
			if err := s.cloudflareSync(); err != nil {
				s.logger.Error("failed to sync Cloudflare hostnames", slog.Any("err", err))
			}
		case <-s.ctx.Done():
			s.logger.Info("Stopping Cloudflare sync ticker")
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
			s.logger.Error("failed to process service", slog.String("service", service.Spec.Name), slog.Any("err", err))
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

	hosts, port, err := s.getDockerServiceMetadata(serviceName)
	if err != nil {
		slog.Debug("skipping service", slog.String("service", serviceName), slog.String("reason", err.Error()))
		return nil
	}

	if len(hosts) == 0 {
		slog.Debug("service has no hostnames configured",
			slog.String("service", serviceName))
		return nil
	}

	slog.Debug("processing service with hosts",
		slog.String("service", serviceName),
		slog.Int("host_count", len(hosts)),
		slog.Any("hosts", hosts))

	internalServiceURL := fmt.Sprintf("http://%s:%s", serviceName, port)
	for _, h := range hosts {

		existingConfig, exists := existingConfigs[h]

		if err := s.cloudflareClient.updateTunnelConfig(s.ctx, h, internalServiceURL); err != nil {
			return fmt.Errorf("failed to update tunnel config for %s: %w", h, err)
		}

		isNew := !exists
		isChanged := exists && existingConfig.URL != internalServiceURL

		if isNew {
			s.logger.Debug("created new tunnel rule", slog.String("hostname", h), slog.String("service", internalServiceURL))
			zoneID, err := s.cloudflareClient.getZoneID(s.ctx, h)
			if err != nil {
				return fmt.Errorf("failed to get zone ID for hostname %s: %w", h, err)
			}

			err = s.cloudflareClient.createTunnelDNSRecord(s.ctx, zoneID, h)
			if err != nil {
				return fmt.Errorf("failed to create DNS record for hostname %s: %w", h, err)
			}

			s.logger.Debug("created DNS record", slog.String("hostname", h), slog.String("zoneID", zoneID))
		} else if isChanged {
			s.logger.Debug("updated existing tunnel rule", slog.String("hostname", h),
				slog.String("old_service", existingConfig.URL),
				slog.String("new_service", internalServiceURL))
		} else {
			s.logger.Debug("tunnel rule already exists and is up-to-date", slog.String("hostname", h))
		}
	}

	return nil
}
