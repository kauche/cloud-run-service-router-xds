package xds

import (
	"context"

	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"

	"github.com/kauche/cloud-run-service-router-xds/internal/domain/distributor"
	"github.com/kauche/cloud-run-service-router-xds/internal/domain/entity"
)

var _ distributor.ServiceDistributor = (*ServiceDistributor)(nil)

type ServiceDistributor struct {
	snapshotCache cache.SnapshotCache
}

func NewServiceDistributor(sc cache.SnapshotCache) *ServiceDistributor {
	return &ServiceDistributor{
		snapshotCache: sc,
	}
}

func (s *ServiceDistributor) DistributeServices(ctx context.Context, services []*entity.Service) error {
	for _, nodeID := range s.snapshotCache.GetStatusKeys() {
		// TODO:
		if err := s.snapshotCache.SetSnapshot(ctx, nodeID, nil); err != nil {
			// TODO:
		}
	}

	return nil
}
