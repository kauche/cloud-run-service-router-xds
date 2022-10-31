package distributor

import (
	"context"

	"github.com/kauche/cloud-run-service-router-xds/internal/domain/entity"
)

type ServiceDistributor interface {
	DistributeServices(ctx context.Context, services []*entity.Service) error
}
