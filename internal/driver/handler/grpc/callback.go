package grpc

import (
	"context"
	"errors"
	"fmt"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	server "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/go-logr/logr"

	"github.com/kauche/cloud-run-service-router-xds/internal/usecase"
)

var _ server.Callbacks = (*callbacks)(nil)

type callbacks struct {
	uc            usecase.ServiceUseCase
	snapshotCache cache.SnapshotCache
	logger        logr.Logger
}

func (c *callbacks) OnStreamOpen(_ context.Context, streamID int64, _ string) error {
	c.logger.Info("stream opened", "streamID", streamID)
	return nil
}

func (c *callbacks) OnStreamClosed(streamID int64, node *core.Node) {
	c.logger.Info("stream closed", "streamID", streamID)
}

func (c *callbacks) OnStreamRequest(streamID int64, req *discovery.DiscoveryRequest) error {
	c.logger.Info("stream request", "type", req.TypeUrl, "streamID", streamID, "request", req.ResourceNames, "version", req.VersionInfo)

	node := req.GetNode()
	if node == nil {
		return errors.New("node does not exist on the request")
	}

	ctx := context.Background()

	switch req.TypeUrl {
	case resource.ListenerType:
		if err := c.uc.RegisterClientToDistributor(ctx, node.Id, req.ResourceNames); err != nil {
			c.logger.Error(err, "failed to register the client to distributor", "streamID", streamID, "node", node.Id)
			return fmt.Errorf("failed to register the client to the distributor: %w", err)
		}

		if err := c.uc.DistributeServicesToClient(ctx, node.Id, req.ResourceNames); err != nil {
			c.logger.Error(err, "failed to distribute services to the client", "streamID", streamID, "node", node.Id)
			return fmt.Errorf("failed to distribute services to the client: %w", err)
		}
	case resource.ClusterType:
		if err := c.uc.RegisterClustersToDistributor(ctx, node.Id, req.ResourceNames); err != nil {
			c.logger.Error(err, "failed to register the clusters to distributor", "streamID", streamID, "node", node.Id)
			return fmt.Errorf("failed to register the clusters to the distributor: %w", err)
		}

		if err := c.uc.DistributeClustersToClient(ctx, node.Id, req.ResourceNames); err != nil {
			c.logger.Error(err, "failed to distribute clusters to the client", "streamID", streamID, "node", node.Id)
			return fmt.Errorf("failed to distribute clusters to the client: %w", err)
		}
	}

	return nil
}

func (c *callbacks) OnStreamResponse(_ context.Context, streamID int64, req *discovery.DiscoveryRequest, res *discovery.DiscoveryResponse) {
	c.logger.Info("stream response", "streamID", streamID, "request", req, "response", res)
}

func (c callbacks) OnFetchRequest(_ context.Context, req *discovery.DiscoveryRequest) error {
	c.logger.Info("fetch request")
	return errors.New("fetch version of xDS is not supported")
}

func (c *callbacks) OnFetchResponse(req *discovery.DiscoveryRequest, _ *discovery.DiscoveryResponse) {
	c.logger.Info("fetch response")
}

func (c *callbacks) OnDeltaStreamOpen(_ context.Context, streamID int64, _ string) error {
	c.logger.Info("delta stream opened", "streamID", streamID)
	return errors.New("delta version of xDS is not supported")
}

func (c *callbacks) OnDeltaStreamClosed(streamID int64, node *core.Node) {
	c.logger.Info("delta stream closed", "streamID", streamID)
}

func (c *callbacks) OnStreamDeltaRequest(streamID int64, _ *discovery.DeltaDiscoveryRequest) error {
	c.logger.Info("delta stream requested", "streamID", streamID)
	return errors.New("delta version of xDS is not supported")
}

func (c *callbacks) OnStreamDeltaResponse(streamID int64, _ *discovery.DeltaDiscoveryRequest, _ *discovery.DeltaDiscoveryResponse) {
	c.logger.Info("delta stream response", "streamID", streamID)
}
