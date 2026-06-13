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

	// UnregisterClient removes the client's listener subscription and clears
	// any snapshot stored under the client key. Used when the ADS stream
	// associated with the client closes.
	UnregisterClient(ctx context.Context, client string) error
}
