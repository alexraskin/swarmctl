package server

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/alexraskin/swarmctl/internal/metrics"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
)

const (
	cooldown = 1 * time.Minute

	// reconnectDelay is how long to wait before re-subscribing after Docker
	// closes an event stream.
	reconnectDelay = 5 * time.Second

	syncMaxRetries = 5
	syncBaseDelay  = 5 * time.Second
)

// syncServiceWithRetry pushes a service's tunnel config to Cloudflare, retrying
// with exponential backoff. Runs in its own goroutine; never blocks the event
// loop. Aborts on context cancellation.
func (s *Server) syncServiceWithRetry(svc *swarm.Service, name string) {
	delay := syncBaseDelay
	for attempt := 1; attempt <= syncMaxRetries; attempt++ {
		start := time.Now()
		err := s.cfSyncer.SyncService(s.ctx, svc)
		duration := time.Since(start).Seconds()

		if err == nil {
			metrics.RecordCloudflareSync("success", duration)
			s.logger.Debug("Cloudflare sync succeeded", slog.String("service", name))
			return
		}

		metrics.RecordCloudflareSync("error", duration)
		s.logger.Error("Cloudflare sync failed", slog.String("service", name), slog.Int("attempt", attempt), "error", err)

		if attempt == syncMaxRetries {
			return
		}

		select {
		case <-s.ctx.Done():
			return
		case <-time.After(delay):
			delay *= 2
		}
	}
}

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

// consumeDockerEvents subscribes to Docker events matching filter and invokes
// handle for every message until the server context is cancelled. If Docker
// closes the stream it re-subscribes after a short delay, so each monitor is a
// single long-lived goroutine rather than a chain of respawned ones.
func (s *Server) consumeDockerEvents(name string, filter filters.Args, handle func(events.Message)) {
	for {
		msgs, errs := s.dockerClient.GetDockerEvents(s.ctx, filter)

	stream:
		for {
			select {
			case <-s.ctx.Done():
				return
			case err, ok := <-errs:
				if !ok {
					s.logger.Warn("Docker event stream closed, reconnecting", slog.String("monitor", name))
					break stream
				}
				if err != nil {
					s.logger.Error("Docker event error", slog.String("monitor", name), "error", err)
				}
			case msg := <-msgs:
				handle(msg)
			}
		}

		select {
		case <-s.ctx.Done():
			return
		case <-time.After(reconnectDelay):
		}
	}
}

func (s *Server) monitorServiceEvents() {
	filter := filters.NewArgs()
	filter.Add("type", "service")
	filter.Add("event", "create")
	filter.Add("event", "update")
	s.consumeDockerEvents("service-events", filter, s.handleServiceEvent)
}

func (s *Server) handleServiceEvent(msg events.Message) {
	name := msg.Actor.Attributes["name"]
	metrics.RecordDockerEvent(string(msg.Action), name)

	svc, err := s.dockerClient.GetDockerService(name, s.ctx)
	if err != nil {
		s.logger.Error("Fetch service failed", slog.String("service", name), "error", err)
		return
	}
	if svc.Spec.Labels["cloudflared.tunnel.enabled"] != "true" {
		s.logger.Debug("Service is not enabled for Cloudflare tunnel", slog.String("service", name))
		return
	}

	// Cache the hostnames for this service for later removal
	hostnames := s.extractHostnames(svc.Spec.Labels)
	if len(hostnames) > 0 {
		s.serviceHostnames.Store(name, hostnames)
	}

	// Sync off the event loop so a slow/failing Cloudflare call does not stall
	// processing of other service events.
	go s.syncServiceWithRetry(svc, name)
}

func (s *Server) monitorContainerEvents() {
	filter := filters.NewArgs()
	filter.Add("type", "container")
	filter.Add("event", "die")
	filter.Add("event", "restart")
	filter.Add("event", "crash")
	s.consumeDockerEvents("container-events", filter, s.handleContainerEvent)
}

func (s *Server) handleContainerEvent(msg events.Message) {
	containerID := msg.Actor.ID
	if len(containerID) > 12 {
		containerID = containerID[:12]
	}
	status := msg.Action
	name := msg.Actor.Attributes["name"]
	exitCode := msg.Actor.Attributes["exitCode"]

	eventKey := fmt.Sprintf("%s:%s", containerID, status)
	now := time.Now()
	if lastSeenRaw, exists := s.recentEvents.Load(eventKey); exists {
		if lastSeen, ok := lastSeenRaw.(time.Time); ok && now.Sub(lastSeen) < cooldown {
			return
		}
	}
	s.recentEvents.Store(eventKey, now)

	if s.notifier.Enabled() {
		body := fmt.Sprintf("Container has died or restarted: %s (%s) with exit code %s", name, containerID, exitCode)

		if err := s.notifier.Send("DOCKER SWARM EVENT", body); err != nil {
			metrics.RecordNotification("error")
			s.logger.Error("Error sending notification", "error", err)
		} else {
			metrics.RecordNotification("success")
		}
	}

	s.logger.Debug("Container event", "name", name, "containerID", containerID, "status", status, "exitCode", exitCode, "timestamp", time.Unix(msg.Time, 0).Format(time.RFC3339))
}

func (s *Server) runEventCleanup(interval, maxAge time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

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
			return
		}
	}
}

func (s *Server) monitorServiceRemovals() {
	filter := filters.NewArgs()
	filter.Add("type", "service")
	filter.Add("event", "remove")
	s.consumeDockerEvents("service-removals", filter, s.handleServiceRemoval)
}

func (s *Server) handleServiceRemoval(msg events.Message) {
	name := msg.Actor.Attributes["name"]
	metrics.RecordDockerEvent(string(msg.Action), name)

	// Only tunnel-enabled services (seen during create/update) need reconciling.
	if _, exists := s.serviceHostnames.Load(name); !exists {
		s.logger.Debug("Service removed but was not tunnel-enabled", slog.String("service", name))
		return
	}

	s.logger.Debug("Tunnel-enabled service removed, will reconcile after delay", slog.String("service", name), slog.Int("delay_minutes", s.config.ServiceRemovalDelayMinutes))
	s.pendingRemovals.Store(name, pendingRemoval{
		ServiceName: name,
		RemovedAt:   time.Now(),
	})
	// serviceHostnames is cleaned up during reconciliation.
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

		// Drop from the syncer cache so a future recreate re-provisions instead
		// of being treated as already-present (it may also lose its DNS below).
		s.cfSyncer.InvalidateHost(hostname)

		// Remove the Access (SSO) application for this hostname, if one exists.
		if err := s.cfClient.DeleteAccessApp(s.ctx, hostname); err != nil {
			s.logger.Error("Failed to remove access app", "error", err, slog.String("hostname", hostname))
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
