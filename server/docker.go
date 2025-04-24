package server

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
)

type DockerUpdateResponse struct {
	Success    bool   `json:"success"`
	OldVersion uint64 `json:"oldVersion"`
	NewVersion uint64 `json:"newVersion"`
}

func (s *Server) updateDockerService(serviceName string, image string) (*DockerUpdateResponse, error) {

	service, _, err := s.dockerClient.ServiceInspectWithRaw(s.ctx, serviceName, types.ServiceInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service: %v", err)
	}
	oldVersion := service.Version.Index

	service.Spec.TaskTemplate.ContainerSpec.Image = image

	_, err = s.dockerClient.ServiceUpdate(s.ctx, service.ID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update service: %v", err)
	}
	return &DockerUpdateResponse{
		Success:    true,
		OldVersion: oldVersion,
		NewVersion: service.Version.Index,
	}, nil
}

func (s *Server) getDockerServiceMetadata(serviceName string) ([]string, string, error) {
	service, _, err := s.dockerClient.ServiceInspectWithRaw(s.ctx, serviceName, types.ServiceInspectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get service: %w", err)
	}

	labels := service.Spec.Labels
	port, portOK := labels["tunnel.port"]
	host, hostOK := labels["tunnel.hostname"]

	if !portOK && !hostOK {
		return nil, "", fmt.Errorf("service %s has no tunnel configuration", serviceName)
	}

	if !portOK || port == "" {
		return nil, "", fmt.Errorf("service %s missing tunnel.port", serviceName)
	}

	if !hostOK || host == "" {
		return []string{}, port, nil
	}

	hosts := strings.Split(host, ",")
	return hosts, port, nil
}

func (s *Server) getDockerServices() ([]swarm.Service, error) {
	services, err := s.dockerClient.ServiceList(s.ctx, types.ServiceListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get services: %v", err)
	}
	return services, nil
}

func (s *Server) getDockerEvents(eventFilter filters.Args) (<-chan events.Message, <-chan error) {
	return s.dockerClient.Events(s.ctx, events.ListOptions{
		Filters: eventFilter,
	})
}
