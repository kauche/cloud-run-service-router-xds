package repository

import (
	"context"

	"github.com/kauche/cloud-run-service-router-xds/internal/domain/entity"
)

type ServiceRepository interface {
	ListAllServices(ctx context.Context) ([]*entity.Service, error)
	RefreshServices(ctx context.Context) error
}
