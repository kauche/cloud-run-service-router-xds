package usecase

import (
	"context"
	"fmt"

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
		return fmt.Errorf("failed to list services: %w", err)
	}

	if err := u.distributor.DistributeServices(ctx, services); err != nil {
		return fmt.Errorf("failed to distribute services: %w", err)
	}

	return nil
}

func (u *ServiceUseCase) DistributeServicesToClient(ctx context.Context, client string) error {
	services, err := u.repository.ListAllServices(ctx)
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	if err := u.distributor.DistributeServicesToClient(ctx, services, client); err != nil {
		return fmt.Errorf("failed to distribute services to the client `%s`: %w", client, err)
	}

	return nil
}

func (u *ServiceUseCase) RefreshServices(ctx context.Context) error {
	if err := u.repository.RefreshServices(ctx); err != nil {
		return fmt.Errorf("failed to refresh services: %w", err)
	}

	if err := u.broker.PublishServicesRefreshedEvent(ctx); err != nil {
		return fmt.Errorf("failed to publish serivce refreshed event: %w", err)
	}

	return nil
}

func (u *ServiceUseCase) RegisterClientToDistributor(ctx context.Context, client string, serviceNames []string) error {
	if err := u.distributor.RegisterClient(ctx, client, serviceNames); err != nil {
		return fmt.Errorf("failed to add client to the distributor: %w", err)
	}

	return nil
}
