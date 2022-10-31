package cloudrun

import (
	"context"

	"github.com/kauche/cloud-run-service-router-xds/internal/domain/entity"
	"github.com/kauche/cloud-run-service-router-xds/internal/domain/repository"
)

var _ repository.ServiceRepository = (*ServiceRepository)(nil)

type ServiceRepository struct{}

func NewServiceRepository() *ServiceRepository {
	return &ServiceRepository{}
}

func (s *ServiceRepository) ListAllServices(ctx context.Context) ([]*entity.Service, error) {
	panic("not implemented") // TODO: Implement
}

func (s *ServiceRepository) RefreshServices(ctx context.Context) error {
	panic("not implemented") // TODO: Implement
}
