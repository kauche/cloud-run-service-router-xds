package xds

import (
	"context"
	"slices"
	"testing"
	"time"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/stream/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/kauche/cloud-run-service-router-xds/internal/domain/entity"
)

// TestMultiStreamSameNodeID_SubscriptionOverwrite verifies that two distinct
// ADS streams from the same bootstrap `node.id` do not silently overwrite
// each other's subscription state on the control plane side.
//
// Since grpc-go v1.66 ([PR #7347]), every `grpc.NewClient("xds:///...")`
// channel inside a single Go process creates its own xDS client with its own
// ADS stream, but all of those streams continue to carry the same bootstrap
// `node.id`. This control plane keys both subscription maps and the
// SnapshotCache by `node.id` (`cache.IDHash{}` plus
// `clientListenersMu.clientRequestedListeners[node.id]` /
// `clientClustersMu.clientRequestedClusters[node.id]`), so the second stream's
// LDS request silently replaces the first stream's subscription and rewrites
// the only snapshot stored under that node.
//
// This test asserts the post-conditions of that collision:
//
//  1. `clientRequestedListeners[node.id]` still contains the first stream's
//     subscription. If not, the periodic `DistributeServices` broadcast will
//     only ever republish what the second stream asked for, and the first
//     stream will silently stop receiving listener updates for its actual
//     subscription.
//  2. The SnapshotCache entry for that node still contains the first stream's
//     listener. If not, the next time anything in the first stream's xDS
//     pipeline causes the cache to compare versions, the cache will see the
//     listener as gone.
//
// Both assertions FAIL while the bug is present and PASS once the cache key
// is disambiguated per stream (or per-stream subscriptions are unioned before
// publishing the snapshot).
//
// [PR #7347]: https://github.com/grpc/grpc-go/pull/7347
func TestMultiStreamSameNodeID_SubscriptionOverwrite(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sc := cache.NewSnapshotCache(true, cache.IDHash{}, testLogger{})
	sd := NewServiceDistributor(sc)

	services := []*entity.Service{
		newService("origin-service-1"),
		newService("origin-service-2"),
	}
	const nodeID = "shared-bootstrap-node-id"

	// Stream A registers and distributes for origin-service-1.
	if err := sd.RegisterClient(ctx, nodeID, []string{"origin-service-1"}); err != nil {
		t.Fatalf("RegisterClient (stream A): %v", err)
	}
	if err := sd.DistributeServicesToClient(ctx, services, nodeID, []string{"origin-service-1"}); err != nil {
		t.Fatalf("DistributeServicesToClient (stream A): %v", err)
	}
	snapA, err := sc.GetSnapshot(nodeID)
	if err != nil {
		t.Fatalf("GetSnapshot after stream A: %v", err)
	}
	if got := listenerNamesFromSnapshot(snapA); !slices.Contains(got, "origin-service-1") {
		t.Fatalf("precondition violated: stream A's snapshot does not contain origin-service-1, got %v", got)
	}

	// Stream B (same node.id) registers and distributes for origin-service-2.
	if err := sd.RegisterClient(ctx, nodeID, []string{"origin-service-2"}); err != nil {
		t.Fatalf("RegisterClient (stream B): %v", err)
	}
	if err := sd.DistributeServicesToClient(ctx, services, nodeID, []string{"origin-service-2"}); err != nil {
		t.Fatalf("DistributeServicesToClient (stream B): %v", err)
	}

	// Assertion 1: the periodic-refresh subscription map has been overwritten.
	// Stream A's subscription to "origin-service-1" must still be retrievable
	// because Stream A's ADS stream is still open and still subscribed; the
	// broadcast tick is the only place that re-pushes a snapshot for a node,
	// and it iterates this map.
	sd.clientListenersMu.RLock()
	got := append([]string(nil), sd.clientListenersMu.clientRequestedListeners[nodeID]...)
	sd.clientListenersMu.RUnlock()
	if !slices.Contains(got, "origin-service-1") {
		t.Errorf("multi-stream subscription overwrite: clientRequestedListeners[%q] no longer contains stream A's subscription %q after stream B (same node.id) subscribed to %q; got %v. "+
			"The next periodic DistributeServices tick will rebuild stream A's snapshot from this list, so stream A will silently stop receiving listener updates for its actual subscription.",
			nodeID, "origin-service-1", "origin-service-2", got)
	}

	// Assertion 2: the snapshot stored under node.id has lost Stream A's
	// listener. The next time anything in stream A's xDS pipeline causes the
	// cache to compare versions (e.g. a redelivery on stream A's tracked
	// watch, or a fresh CreateWatch on a reconnect), the cache will see the
	// listener as gone.
	snapB, err := sc.GetSnapshot(nodeID)
	if err != nil {
		t.Fatalf("GetSnapshot after stream B: %v", err)
	}
	listeners := listenerNamesFromSnapshot(snapB)
	if !slices.Contains(listeners, "origin-service-1") {
		t.Errorf("multi-stream snapshot overwrite: the SnapshotCache entry for node.id %q no longer contains stream A's listener %q after stream B subscribed under the same node.id to %q; got listeners=%v. "+
			"Because go-control-plane's SnapshotCache holds exactly one snapshot per node hash, the only snapshot stored for both streams is now stream B's view.",
			nodeID, "origin-service-1", "origin-service-2", listeners)
	}
}

// TestMultiStreamSameNodeID_TriggersResourceRemoved demonstrates the
// user-visible symptom of the multi-stream collision: a tracked SotW watch
// from stream A gets fed a snapshot from stream B that does not include
// stream A's subscribed listener, and the cache dispatches a SotW LDS
// response from which grpc-go's xdsclient concludes the listener has been
// removed. The xDS resolver then pushes an erroring config selector to the
// channel and the next RPC fails with `Unavailable`.
//
// To bypass go-control-plane's ADS-mode "missing request resource" guard,
// which silently drops responses when the snapshot contains resource names
// that the watch did NOT subscribe to, this test uses a configuration where
// stream B's subscription is a SUBSET of stream A's. In production this
// happens whenever multiple grpc-go targets in the same process map to
// overlapping cluster sets, or whenever stream A subscribed (in a previous
// build / a non-grpc-go client) to a superset of stream B's resources.
//
// This test FAILS while the bug is present (stream A receives a SotW response
// that omits one of its subscribed listeners) and PASSES once subscriptions
// from distinct streams on the same node.id are disambiguated.
func TestMultiStreamSameNodeID_TriggersResourceRemoved(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sc := cache.NewSnapshotCache(true, cache.IDHash{}, testLogger{})
	sd := NewServiceDistributor(sc)

	services := []*entity.Service{
		newService("origin-service-1"),
		newService("origin-service-2"),
	}
	const nodeID = "shared-bootstrap-node-id"
	node := &core.Node{Id: nodeID}

	// Stream A subscribes to BOTH listeners.
	if err := sd.RegisterClient(ctx, nodeID, []string{"origin-service-1", "origin-service-2"}); err != nil {
		t.Fatalf("RegisterClient (stream A): %v", err)
	}
	if err := sd.DistributeServicesToClient(ctx, services, nodeID, []string{"origin-service-1", "origin-service-2"}); err != nil {
		t.Fatalf("DistributeServicesToClient (stream A): %v", err)
	}
	snap, err := sc.GetSnapshot(nodeID)
	if err != nil {
		t.Fatalf("GetSnapshot after stream A: %v", err)
	}
	versionA := snap.GetVersion(resource.ListenerType)

	// Stream A ACKs the snapshot. createResponse returns (nil, nil) because
	// the snapshot version matches and nothing changed, so SnapshotCache
	// starts tracking the watch — exactly the state in which a subsequent
	// SetSnapshot can dispatch into it.
	streamARespCh := make(chan cache.Response, 1)
	streamAReq := &discovery.DiscoveryRequest{
		Node:          node,
		TypeUrl:       resource.ListenerType,
		ResourceNames: []string{"origin-service-1", "origin-service-2"},
		VersionInfo:   versionA,
	}
	streamASub := stream.NewSotwSubscription([]string{"origin-service-1", "origin-service-2"}, false)
	streamASub.SetReturnedResources(map[string]string{
		"origin-service-1": versionA,
		"origin-service-2": versionA,
	})
	cancelA, err := sc.CreateWatch(streamAReq, streamASub, streamARespCh)
	if err != nil {
		t.Fatalf("CreateWatch (stream A ACK): %v", err)
	}
	defer cancelA()

	// Sanity-check: stream A's tracked watch should currently be silent.
	select {
	case resp := <-streamARespCh:
		dr, err := resp.GetDiscoveryResponse()
		if err != nil {
			t.Fatalf("decode unexpected immediate response on stream A: %v", err)
		}
		t.Fatalf("stream A unexpectedly received an immediate response (%d resources, version=%q) before stream B subscribed",
			len(dr.GetResources()), dr.GetVersionInfo())
	case <-time.After(50 * time.Millisecond):
	}

	// Stream B (same node.id) subscribes only to a subset of stream A's
	// listeners. The collision rebuilds the snapshot for node.id with only
	// stream B's listener.
	if err := sd.RegisterClient(ctx, nodeID, []string{"origin-service-1"}); err != nil {
		t.Fatalf("RegisterClient (stream B): %v", err)
	}
	if err := sd.DistributeServicesToClient(ctx, services, nodeID, []string{"origin-service-1"}); err != nil {
		t.Fatalf("DistributeServicesToClient (stream B): %v", err)
	}

	select {
	case resp := <-streamARespCh:
		dr, err := resp.GetDiscoveryResponse()
		if err != nil {
			t.Fatalf("decode stream A response: %v", err)
		}
		names := listenerNamesFromResources(t, dr.GetResources())
		if !slices.Contains(names, "origin-service-1") || !slices.Contains(names, "origin-service-2") {
			t.Errorf("multi-stream collision triggered phantom resource removal: "+
				"stream A subscribed to %v, stream B subscribed (under the same node.id %q) to %v, "+
				"and stream A then received a SotW LDS DiscoveryResponse that drops one of its subscribed listeners: "+
				"got %d resource(s) %v at version=%q. "+
				"On a real grpc-go client this surfaces as `xds: resource %q of type \"ListenerResource\" has been removed`, "+
				"the xDS resolver pushes an erroring config selector, the channel goes to TRANSIENT_FAILURE, and the next RPC fails.",
				[]string{"origin-service-1", "origin-service-2"},
				nodeID,
				[]string{"origin-service-1"},
				len(dr.GetResources()), names, dr.GetVersionInfo(),
				"origin-service-2")
		}
	case <-time.After(1 * time.Second):
		// Once subscriptions across streams under the same node.id are
		// disambiguated, stream A's watch is never dispatched into from
		// stream B's distribution, so this timeout is the correct outcome.
	}
}

func newService(name string) *entity.Service {
	return &entity.Service{
		Name:    name,
		Version: "v1",
		DefaultRoute: &entity.Route{
			Name: name,
			Host: name + ".example.com",
		},
		Routes: map[string]*entity.Route{},
	}
}

func listenerNamesFromSnapshot(snap cache.ResourceSnapshot) []string {
	out := make([]string, 0)
	for name := range snap.GetResources(resource.ListenerType) {
		out = append(out, name)
	}
	return out
}

func listenerNamesFromResources(t *testing.T, resources []*anypb.Any) []string {
	t.Helper()
	out := make([]string, 0, len(resources))
	for _, r := range resources {
		lis := &listener.Listener{}
		if err := anypb.UnmarshalTo(r, lis, proto.UnmarshalOptions{}); err != nil {
			t.Fatalf("unmarshal listener: %v", err)
		}
		out = append(out, lis.GetName())
	}
	return out
}

type testLogger struct{}

func (testLogger) Debugf(string, ...any) {}
func (testLogger) Infof(string, ...any)  {}
func (testLogger) Warnf(string, ...any)  {}
func (testLogger) Errorf(string, ...any) {}
