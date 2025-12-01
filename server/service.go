package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/alexraskin/swarmctl/internal/metrics"
	"github.com/alexraskin/swarmctl/internal/pushover"
	"github.com/docker/docker/api/types/filters"
)

const (
	cooldown = 1 * time.Minute
	maxAge   = 1 * time.Hour
)

// extractHostnames extracts all hostnames from service labels
func (s *Server) extractHostnames(labels map[string]string) []string {
	hostnames := []string{}

	// Add primary hostname if present
	if primary := labels["cloudflared.tunnel.hostname"]; primary != "" {
		for h := range strings.SplitSeq(primary, ",") {
			if h = strings.TrimSpace(h); h != "" {
				hostnames = append(hostnames, h)
			}
		}
	}

	// Add any additional hostnames from labels ending with .hostname
	for k, v := range labels {
		if k != "cloudflared.tunnel.hostname" && strings.HasSuffix(k, ".hostname") && v != "" {
			for h := range strings.SplitSeq(v, ",") {
				if h = strings.TrimSpace(h); h != "" {
					hostnames = append(hostnames, h)
				}
			}
		}
	}

	return hostnames
}

func (s *Server) startDockerMonitor() error {
	go s.monitorServiceEvents()
	go s.monitorServiceRemovals()
	go func() {
		if err := s.dockerEventsMonitor(); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
			s.logger.Error("Error reading Docker event", "error", err)
		}
	}()
	s.logger.Debug("Docker events monitoring started")
	return nil
}

func (s *Server) monitorServiceEvents() error {
	eventFilter := filters.NewArgs()
	eventFilter.Add("type", "service")
	eventFilter.Add("event", "create")
	eventFilter.Add("event", "update")

	s.logger.Debug("Starting service event monitor")

	for {
		msgs, errs := s.dockerClient.GetDockerEvents(s.ctx, eventFilter)

		for {
			select {
			case <-s.ctx.Done():
				s.logger.Debug("Stopping service monitor")
				return nil

			case err, ok := <-errs:
				if !ok {
					s.logger.Warn("Docker event error channel closed, reconnecting...")
					go s.monitorServiceEvents()
					return nil
				}
				if err != nil {
					s.logger.Error("Error reading Docker event", "error", err)
					time.Sleep(5 * time.Second)
					continue
				}

			case msg := <-msgs:
				name := msg.Actor.Attributes["name"]
				metrics.RecordDockerEvent(string(msg.Action), name)

				svc, err := s.dockerClient.GetDockerService(name, s.ctx)
				if err != nil {
					s.logger.Error("Fetch service failed", slog.String("service", name), "error", err)
					continue
				}
				if svc.Spec.Labels["cloudflared.tunnel.enabled"] != "true" {
					s.logger.Debug("Service is not enabled for Cloudflare tunnel", slog.String("service", name))
					continue
				}

				// Cache the hostnames for this service for later removal
				hostnames := s.extractHostnames(svc.Spec.Labels)
				if len(hostnames) > 0 {
					s.serviceHostnames.Store(name, hostnames)
				}

				start := time.Now()
				if err := s.cfSyncer.SyncService(s.ctx, svc); err != nil {
					duration := time.Since(start).Seconds()
					metrics.RecordCloudflareSync("error", duration)
					s.logger.Error("Cloudflare sync failed", slog.String("service", name), "error", err)
					time.Sleep(5 * time.Second)
					continue
				}
				duration := time.Since(start).Seconds()
				metrics.RecordCloudflareSync("success", duration)
				s.logger.Debug("Cloudflare sync succeeded", slog.String("service", name))
			}
		}
	}
}

func (s *Server) dockerEventsMonitor() error {
	eventFilter := filters.NewArgs()
	eventFilter.Add("type", "container")
	eventFilter.Add("event", "die")
	eventFilter.Add("event", "restart")
	eventFilter.Add("event", "crash")

	for {
		msgs, errs := s.dockerClient.GetDockerEvents(s.ctx, eventFilter)

		for {
			select {
			case err, ok := <-errs:
				if !ok {
					s.logger.Warn("Docker event error channel closed, reconnecting...")
					go s.dockerEventsMonitor()
					return nil
				}
				if err != nil {
					s.logger.Error("Error reading Docker event", "error", err)
				}
				time.Sleep(5 * time.Second)
			case msg := <-msgs:
				containerID := msg.Actor.ID[:12]
				status := msg.Action
				name := msg.Actor.Attributes["name"]
				exitCode := msg.Actor.Attributes["exitCode"]

				eventKey := fmt.Sprintf("%s:%s", containerID, status)
				now := time.Now()

				if lastSeenRaw, exists := s.recentEvents.Load(eventKey); exists {
					lastSeen := lastSeenRaw.(time.Time)
					if now.Sub(lastSeen) < cooldown {
						continue
					}
				}

				s.recentEvents.Store(eventKey, now)

				pushoverMsg := pushover.PushoverMessage{
					Title:     "DOCKER SWARM EVENT",
					Message:   fmt.Sprintf("Container has died or restarted: %s (%s) with exit code %s", name, containerID, exitCode),
					Timestamp: time.Unix(msg.Time, 0).Unix(),
					Recipient: s.config.PushoverRecipient,
				}

				err := s.pushoverClient.SendNotification(pushoverMsg)
				if err != nil {
					metrics.RecordPushoverNotification("error")
					s.logger.Error("Error sending Pushover notification", "error", err)
				} else {
					metrics.RecordPushoverNotification("success")
				}

				s.logger.Debug("Container event", "name", name, "containerID", containerID, "status", status, "exitCode", exitCode, "timestamp", time.Unix(msg.Time, 0).Format(time.RFC3339))
			case <-s.ctx.Done():
				s.logger.Debug("Stopping Docker events monitor")
				return nil
			}
		}
	}
}

func (s *Server) startEventCleanup(interval time.Duration, maxAge time.Duration) {
	ticker := time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-ticker.C:
				now := time.Now()
				s.recentEvents.Range(func(key, value any) bool {
					lastSeen, ok := value.(time.Time)
					if !ok {
						s.recentEvents.Delete(key)
						return true
					}
					if now.Sub(lastSeen) > maxAge {
						s.logger.Debug("Cleaning up old container event", "key", key)
						s.recentEvents.Delete(key)
					}
					return true
				})

			case <-s.ctx.Done():
				s.logger.Debug("Stopping event cleanup")
				ticker.Stop()
				return
			}
		}
	}()
}

func (s *Server) monitorServiceRemovals() error {
	eventFilter := filters.NewArgs()
	eventFilter.Add("type", "service")
	eventFilter.Add("event", "remove")

	for {
		msgs, errs := s.dockerClient.GetDockerEvents(s.ctx, eventFilter)

		for {
			select {
			case <-s.ctx.Done():
				s.logger.Debug("Stopping service removal monitor")
				return nil

			case err, ok := <-errs:
				if !ok {
					s.logger.Warn("Docker removal event error channel closed, reconnecting...")
					go s.monitorServiceRemovals()
					return nil
				}
				if err != nil {
					s.logger.Error("Error reading Docker removal event", "error", err)
					time.Sleep(5 * time.Second)
					continue
				}

			case msg := <-msgs:
				name := msg.Actor.Attributes["name"]
				metrics.RecordDockerEvent(string(msg.Action), name)

				// Check if this service had cloudflare tunnel enabled by looking in our cache
				_, exists := s.serviceHostnames.Load(name)
				if !exists {
					s.logger.Debug("Service removed but was not tunnel-enabled", slog.String("service", name))
					continue
				}

				// Mark the time this service was removed for delayed reconciliation
				s.logger.Debug("Tunnel-enabled service removed, will reconcile after delay", slog.String("service", name), slog.Int("delay_minutes", s.config.ServiceRemovalDelayMinutes))

				s.pendingRemovals.Store(name, pendingRemoval{
					ServiceName: name,
					RemovedAt:   time.Now(),
				})

				// Keep in serviceHostnames cache for now - will be cleaned up during reconciliation
			}
		}
	}
}

func (s *Server) startRemovalProcessor() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	s.logger.Debug("Starting removal processor (reconciliation loop)")

	for {
		select {
		case <-ticker.C:
			// Check if any pending removals are ready for reconciliation
			now := time.Now()
			delay := time.Duration(s.config.ServiceRemovalDelayMinutes) * time.Minute
			shouldReconcile := false

			s.pendingRemovals.Range(func(key, value any) bool {
				removal, ok := value.(pendingRemoval)
				if !ok {
					s.pendingRemovals.Delete(key)
					return true
				}

				// Check if enough time has passed
				if now.Sub(removal.RemovedAt) >= delay {
					shouldReconcile = true
					s.pendingRemovals.Delete(key)
				}
				return true
			})

			if shouldReconcile {
				if err := s.reconcileTunnelConfig(); err != nil {
					s.logger.Error("Failed to reconcile tunnel config", "error", err)
				}
			}

		case <-s.ctx.Done():
			s.logger.Debug("Stopping removal processor")
			return
		}
	}
}

// reconcileTunnelConfig compares tunnel config against running services and removes orphaned entries
func (s *Server) reconcileTunnelConfig() error {

	// 1. Get all running services with tunnel enabled
	services, err := s.dockerClient.GetDockerServices(s.ctx)
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	// Build a set of hostnames that SHOULD exist (from running services)
	desiredHostnames := make(map[string]string) // hostname -> serviceName
	for _, svc := range services {
		if svc.Spec.Labels["cloudflared.tunnel.enabled"] != "true" {
			continue
		}

		hostnames := s.extractHostnames(svc.Spec.Labels)
		for _, hostname := range hostnames {
			desiredHostnames[hostname] = svc.Spec.Name
		}
	}

	// 2. Get current tunnel config
	tunnelConfig, err := s.cfSyncer.LoadExisting(s.ctx)
	if err != nil {
		return fmt.Errorf("failed to load tunnel config: %w", err)
	}

	// 3. Find hostnames in tunnel config that shouldn't be there
	orphanedHostnames := []string{}
	for hostname := range tunnelConfig {
		if hostname == "" {
			continue
		}
		if _, exists := desiredHostnames[hostname]; !exists {
			orphanedHostnames = append(orphanedHostnames, hostname)
		}
	}

	if len(orphanedHostnames) == 0 {
		s.logger.Debug("No orphaned tunnel configs found")
		return nil
	}

	// 4. Remove orphaned hostnames
	for _, hostname := range orphanedHostnames {
		s.logger.Debug("Removing orphaned tunnel config", slog.String("hostname", hostname))

		// Remove from tunnel config by updating with empty serviceURL
		if err := s.cfClient.UpdateTunnelConfig(s.ctx, hostname, ""); err != nil {
			s.logger.Error("Failed to remove tunnel config", "error", err, slog.String("hostname", hostname))
			continue
		}

		// Optionally delete DNS record
		if s.config.DeleteDNSOnRemoval {
			zoneID, err := s.cfClient.GetZoneID(s.ctx, hostname)
			if err != nil {
				s.logger.Error("Failed to get zone for DNS deletion", "error", err, slog.String("hostname", hostname))
				continue
			}

			recordID, err := s.cfClient.GetTunnelDNSRecord(s.ctx, zoneID, hostname)
			if err != nil {
				s.logger.Debug("DNS record not found (may not exist)", slog.String("hostname", hostname))
				continue
			}

			if err := s.cfClient.DeleteTunnelDNSRecord(s.ctx, recordID, zoneID); err != nil {
				s.logger.Error("Failed to delete DNS record", "error", err, slog.String("hostname", hostname))
			} else {
				s.logger.Debug("Successfully deleted DNS record", slog.String("hostname", hostname))
			}
		}

		s.logger.Debug("Successfully removed orphaned tunnel config", slog.String("hostname", hostname))
	}

	s.logger.Debug("Tunnel config reconciliation complete", slog.Int("removed", len(orphanedHostnames)))
	return nil
}
