package subscriber

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	"github.com/kauche/cloud-run-service-router-xds/internal/usecase"
)

type ServiceEventSubscriber struct {
	uc     *usecase.ServiceUseCase
	logger logr.Logger
}

func NewServiceEventSubscriber(uc *usecase.ServiceUseCase, logger logr.Logger) *ServiceEventSubscriber {
	return &ServiceEventSubscriber{
		uc:     uc,
		logger: logger,
	}
}

func (s *ServiceEventSubscriber) ServicesRefreshedEventHandler() error {
	s.logger.Info("refreshing services")

	ctx := context.Background()

	if err := s.uc.DistributeServices(ctx); err != nil {
		return fmt.Errorf("failed to distribute services: %w", err)
	}

	return nil
}
