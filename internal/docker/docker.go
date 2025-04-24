package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

type DockerClient struct {
	dockerClient *client.Client
}

func NewDockerClient() (*DockerClient, error) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %v", err)
	}
	return &DockerClient{
		dockerClient: dockerClient,
	}, nil
}

type DockerUpdateResponse struct {
	Success    bool   `json:"success"`
	OldVersion uint64 `json:"oldVersion"`
	NewVersion uint64 `json:"newVersion"`
}

func (s *DockerClient) UpdateDockerService(serviceName string, image string, ctx context.Context) (*DockerUpdateResponse, error) {

	service, _, err := s.dockerClient.ServiceInspectWithRaw(ctx, serviceName, types.ServiceInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service: %v", err)
	}
	oldVersion := service.Version.Index

	service.Spec.TaskTemplate.ContainerSpec.Image = image

	_, err = s.dockerClient.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update service: %v", err)
	}
	return &DockerUpdateResponse{
		Success:    true,
		OldVersion: oldVersion,
		NewVersion: service.Version.Index,
	}, nil
}

func (s *DockerClient) GetDockerServiceMetadata(serviceName string, ctx context.Context) ([]string, string, error) {
	service, _, err := s.dockerClient.ServiceInspectWithRaw(ctx, serviceName, types.ServiceInspectOptions{})
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

func (s *DockerClient) GetDockerServices(ctx context.Context) ([]swarm.Service, error) {
	services, err := s.dockerClient.ServiceList(ctx, types.ServiceListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get services: %v", err)
	}
	return services, nil
}

func (s *DockerClient) GetDockerEvents(ctx context.Context, eventFilter filters.Args) (<-chan events.Message, <-chan error) {
	return s.dockerClient.Events(ctx, events.ListOptions{
		Filters: eventFilter,
	})
}
