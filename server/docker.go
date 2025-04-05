package server

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
)

type DockerUpdateResponse struct {
	Success    bool   `json:"success"`
	OldVersion uint64 `json:"old_version"`
	NewVersion uint64 `json:"new_version"`
}

func (s *Server) updateDockerService(ctx context.Context, serviceName string, image string) (*DockerUpdateResponse, error) {

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
