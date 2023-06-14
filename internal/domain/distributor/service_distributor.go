package distributor

import (
	"context"

	"github.com/kauche/cloud-run-service-router-xds/internal/domain/entity"
)

type ServiceDistributor interface {
	DistributeServices(ctx context.Context, services []*entity.Service) error
	DistributeServicesToClient(ctx context.Context, services []*entity.Service, client string, resourceNames []string) error
	DistributeClustersToClient(ctx context.Context, services []*entity.Service, client string, resourceNames []string) error
	RegisterClient(ctx context.Context, client string, serviceNames []string) error
	RegisterClustersToClient(ctx context.Context, client string, serviceNames []string) error
}
