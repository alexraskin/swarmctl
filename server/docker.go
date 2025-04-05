package server

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
)

func (s *Server) updateDockerService(ctx context.Context, serviceName string, image string) error {

	service, _, err := s.dockerClient.ServiceInspectWithRaw(ctx, serviceName, types.ServiceInspectOptions{})
	if err != nil {
		return fmt.Errorf("failed to get service: %v", err)
	}

	service.Spec.TaskTemplate.ContainerSpec.Image = image

	_, err = s.dockerClient.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update service: %v", err)
	}

	fmt.Println("Service updated successfully!")
	return nil
}
