package usecase

import (
	"context"

	"github.com/kauche/cloud-run-service-router-xds/internal/domain/distributor"
	"github.com/kauche/cloud-run-service-router-xds/internal/domain/event"
	"github.com/kauche/cloud-run-service-router-xds/internal/domain/repository"
)

type ServiceUseCase struct {
	broker      event.ServiceEventBroker
	distributor distributor.ServiceDistributor
	repository  repository.ServiceRepository
}

func NewServiceUseCase(broker event.ServiceEventBroker, distributor distributor.ServiceDistributor, repository repository.ServiceRepository) *ServiceUseCase {
	return &ServiceUseCase{
		broker:      broker,
		distributor: distributor,
		repository:  repository,
	}
}

func (u *ServiceUseCase) DistributeServices(ctx context.Context) error {
	services, err := u.repository.ListAllServices(ctx)
	if err != nil {
		// TODO:
	}

	if err := u.distributor.DistributeServices(ctx, services); err != nil {
		// TODO:
	}

	return nil
}

func (u *ServiceUseCase) RefreshServices(ctx context.Context) error {
	if err := u.repository.RefreshServices(ctx); err != nil {
		// TODO:
	}

	if err := u.broker.PublishServicesRefreshedEvent(ctx); err != nil {
		// TODO:
	}

	return nil
}
