package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/alexraskin/swarmctl/internal/metrics"
	"github.com/alexraskin/swarmctl/internal/pushover"
	"github.com/docker/docker/api/types/filters"
)

const (
	cooldown = 1 * time.Minute
	maxAge   = 1 * time.Hour
)

func (s *Server) startDockerMonitor() error {
	go s.monitorServiceEvents()
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
