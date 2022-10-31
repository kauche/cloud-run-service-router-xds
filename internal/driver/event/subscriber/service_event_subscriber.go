package subscriber

import (
	"context"

	"github.com/kauche/cloud-run-service-router-xds/internal/usecase"
)

type ServiceEventSubscriber struct {
	uc *usecase.ServiceUseCase
}

func NewServiceEventSubscriber(uc *usecase.ServiceUseCase) *ServiceEventSubscriber {
	return &ServiceEventSubscriber{
		uc: uc,
	}
}

func (s *ServiceEventSubscriber) ServicesRefreshedEventHandler(ctx context.Context) error {
	if err := s.uc.DistributeServices(ctx); err != nil {
		// TODO:
	}

	return nil
}
