package grpc

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	server "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/go-logr/logr"

	"github.com/kauche/cloud-run-service-router-xds/internal/usecase"
)

var _ server.Callbacks = (*callbacks)(nil)

// streamKeyPrefix prefixes every per-stream cache/distributor key allocated
// by OnStreamOpen so that the key cannot collide with a real upstream
// node.id. Multiple ADS streams from the same Go process carry the same
// bootstrap node.id (the default condition since grpc-go v1.66, which
// builds one xDS client per `grpc.NewClient("xds:///...")` while keeping
// node.id per-process), so we cannot key the SnapshotCache or the
// subscription maps on node.id alone.
const streamKeyPrefix = "stream:"

type callbacks struct {
	uc            usecase.ServiceUseCase
	snapshotCache cache.SnapshotCache
	logger        logr.Logger

	// streamKeysMu protects streamKeys. The map records the per-stream key
	// assigned in OnStreamOpen so that OnStreamRequest can rewrite the
	// incoming Node.Id with it (which is what the SnapshotCache and the
	// distributor's subscription maps end up keying on) and OnStreamClosed
	// can release the corresponding cache and subscription entries.
	streamKeysMu sync.RWMutex
	streamKeys   map[int64]string
}

func streamKey(streamID int64) string {
	return streamKeyPrefix + strconv.FormatInt(streamID, 10)
}

func (c *callbacks) OnStreamOpen(_ context.Context, streamID int64, _ string) error {
	key := streamKey(streamID)

	c.streamKeysMu.Lock()
	c.streamKeys[streamID] = key
	c.streamKeysMu.Unlock()

	c.logger.Info("stream opened", "streamID", streamID, "streamKey", key)
	return nil
}

func (c *callbacks) OnStreamClosed(streamID int64, _ *core.Node) {
	c.streamKeysMu.Lock()
	key, ok := c.streamKeys[streamID]
	delete(c.streamKeys, streamID)
	c.streamKeysMu.Unlock()

	c.logger.Info("stream closed", "streamID", streamID, "streamKey", key)

	if !ok {
		return
	}

	// Drop the per-stream subscription state and clear the snapshot the
	// distributor stored under this key. Without this, the periodic
	// DistributeServices broadcast would keep republishing a snapshot for
	// a stream that no longer exists.
	if err := c.uc.UnregisterClientFromDistributor(context.Background(), key); err != nil {
		c.logger.Error(err, "failed to unregister the client from the distributor", "streamID", streamID, "streamKey", key)
	}
}

func (c *callbacks) OnStreamRequest(streamID int64, req *discovery.DiscoveryRequest) error {
	node := req.GetNode()
	if node == nil {
		return errors.New("node does not exist on the request")
	}

	c.streamKeysMu.RLock()
	key, ok := c.streamKeys[streamID]
	c.streamKeysMu.RUnlock()
	if !ok {
		return fmt.Errorf("no stream key registered for streamID %d", streamID)
	}

	originalNodeID := node.GetId()

	c.logger.Info("stream request",
		"type", req.TypeUrl,
		"streamID", streamID,
		"nodeID", originalNodeID,
		"streamKey", key,
		"request", req.ResourceNames,
		"version", req.VersionInfo,
	)

	// Rewrite Node.Id to the per-stream key BEFORE go-control-plane's
	// SnapshotCache sees the request via CreateWatch, so the default
	// IDHash (which hashes on Node.Id) treats each ADS stream as a
	// distinct logical client even when several streams carry the same
	// bootstrap node.id. The same key is used as the distributor's
	// subscription-map key below.
	//
	// Mutating node.Id is safe here: the only fields the rest of the
	// pipeline reads from Node are the Id (which we are intentionally
	// overriding) and there is no later code path that depends on the
	// original Id for this stream.
	node.Id = key

	ctx := context.Background()

	switch req.TypeUrl {
	case resource.ListenerType:
		if err := c.uc.RegisterClientToDistributor(ctx, key, req.ResourceNames); err != nil {
			c.logger.Error(err, "failed to register the client to distributor", "streamID", streamID, "streamKey", key)
			return fmt.Errorf("failed to register the client to the distributor: %w", err)
		}

		if err := c.uc.DistributeServicesToClient(ctx, key, req.ResourceNames); err != nil {
			c.logger.Error(err, "failed to distribute services to the client", "streamID", streamID, "streamKey", key)
			return fmt.Errorf("failed to distribute services to the client: %w", err)
		}
	case resource.ClusterType:
		if err := c.uc.RegisterClustersToDistributor(ctx, key, req.ResourceNames); err != nil {
			c.logger.Error(err, "failed to register the clusters to distributor", "streamID", streamID, "streamKey", key)
			return fmt.Errorf("failed to register the clusters to the distributor: %w", err)
		}

		if err := c.uc.DistributeClustersToClient(ctx, key, req.ResourceNames); err != nil {
			c.logger.Error(err, "failed to distribute clusters to the client", "streamID", streamID, "streamKey", key)
			return fmt.Errorf("failed to distribute clusters to the client: %w", err)
		}
	}

	return nil
}

func (c *callbacks) OnStreamResponse(_ context.Context, streamID int64, req *discovery.DiscoveryRequest, res *discovery.DiscoveryResponse) {
	c.logger.Info("stream response", "streamID", streamID, "request", req, "response", res)
}

func (c *callbacks) OnFetchRequest(_ context.Context, req *discovery.DiscoveryRequest) error {
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
