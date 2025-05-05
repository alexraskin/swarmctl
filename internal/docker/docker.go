package docker

import (
	"context"
	"fmt"

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

func (d *DockerClient) GetDockerService(serviceName string, ctx context.Context) (*swarm.Service, error) {
	service, _, err := d.dockerClient.ServiceInspectWithRaw(ctx, serviceName, types.ServiceInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service: %v", err)
	}
	return &service, nil
}

func (d *DockerClient) UpdateDockerService(serviceName string, image string, ctx context.Context) (*DockerUpdateResponse, error) {

	service, err := d.GetDockerService(serviceName, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get service: %v", err)
	}

	oldVersion := service.Version.Index

	service.Spec.TaskTemplate.ContainerSpec.Image = image

	_, err = d.dockerClient.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update service: %v", err)
	}
	return &DockerUpdateResponse{
		Success:    true,
		OldVersion: oldVersion,
		NewVersion: service.Version.Index,
	}, nil
}

func (d *DockerClient) GetDockerServices(ctx context.Context) ([]swarm.Service, error) {
	services, err := d.dockerClient.ServiceList(ctx, types.ServiceListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get services: %v", err)
	}
	return services, nil
}

func (d *DockerClient) GetDockerEvents(ctx context.Context, eventFilter filters.Args) (<-chan events.Message, <-chan error) {
	return d.dockerClient.Events(ctx, events.ListOptions{
		Filters: eventFilter,
	})
}
