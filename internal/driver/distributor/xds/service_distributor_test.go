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
	stream "github.com/envoyproxy/go-control-plane/pkg/server/stream/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/kauche/cloud-run-service-router-xds/internal/domain/entity"
)

// TestDistributor_PreservesPerStreamSubscriptions verifies that when two
// distinct keys are registered (as the gRPC server's OnStreamRequest does for
// each ADS stream by deriving a stream-scoped key in OnStreamOpen), the
// distributor stores each stream's subscription independently and the
// SnapshotCache holds a separate snapshot per stream.
//
// Background: since grpc-go v1.66 every `grpc.NewClient("xds:///...")`
// channel in a process owns its own ADS stream, but the bootstrap `node.id`
// is process-wide and therefore shared across all of those streams. Keying
// the SnapshotCache (`cache.IDHash{}`) and the distributor's subscription
// maps on `node.id` alone would collapse those streams to a single entry,
// causing the second stream's subscription to overwrite the first.
// OnStreamOpen mints a unique stream-scoped key and OnStreamRequest rewrites
// `Node.Id` with it before the cache and the distributor see the request, so
// the two streams are routed independently. This test exercises that
// post-condition at the distributor level.
func TestDistributor_PreservesPerStreamSubscriptions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sc := cache.NewSnapshotCache(true, cache.IDHash{}, testLogger{})
	sd := NewServiceDistributor(sc)

	services := []*entity.Service{
		newService("origin-service-1"),
		newService("origin-service-2"),
	}

	const streamKeyA = "stream:1"
	const streamKeyB = "stream:2"

	if err := sd.RegisterClient(ctx, streamKeyA, []string{"origin-service-1"}); err != nil {
		t.Fatalf("RegisterClient (stream A): %v", err)
	}
	if err := sd.DistributeServicesToClient(ctx, services, streamKeyA, []string{"origin-service-1"}); err != nil {
		t.Fatalf("DistributeServicesToClient (stream A): %v", err)
	}
	if err := sd.RegisterClient(ctx, streamKeyB, []string{"origin-service-2"}); err != nil {
		t.Fatalf("RegisterClient (stream B): %v", err)
	}
	if err := sd.DistributeServicesToClient(ctx, services, streamKeyB, []string{"origin-service-2"}); err != nil {
		t.Fatalf("DistributeServicesToClient (stream B): %v", err)
	}

	sd.clientListenersMu.RLock()
	gotA := append([]string(nil), sd.clientListenersMu.clientRequestedListeners[streamKeyA]...)
	gotB := append([]string(nil), sd.clientListenersMu.clientRequestedListeners[streamKeyB]...)
	sd.clientListenersMu.RUnlock()

	if !slices.Contains(gotA, "origin-service-1") {
		t.Errorf("stream A's subscription was lost: clientRequestedListeners[%q]=%v, want it to contain %q",
			streamKeyA, gotA, "origin-service-1")
	}
	if !slices.Contains(gotB, "origin-service-2") {
		t.Errorf("stream B's subscription was lost: clientRequestedListeners[%q]=%v, want it to contain %q",
			streamKeyB, gotB, "origin-service-2")
	}

	snapA, err := sc.GetSnapshot(streamKeyA)
	if err != nil {
		t.Fatalf("GetSnapshot(%q): %v", streamKeyA, err)
	}
	if names := listenerNamesFromSnapshot(snapA); !slices.Contains(names, "origin-service-1") {
		t.Errorf("snapshot for %q does not contain stream A's listener: got listeners=%v", streamKeyA, names)
	}

	snapB, err := sc.GetSnapshot(streamKeyB)
	if err != nil {
		t.Fatalf("GetSnapshot(%q): %v", streamKeyB, err)
	}
	if names := listenerNamesFromSnapshot(snapB); !slices.Contains(names, "origin-service-2") {
		t.Errorf("snapshot for %q does not contain stream B's listener: got listeners=%v", streamKeyB, names)
	}
}

// TestDistributor_DoesNotDispatchAcrossStreams verifies that, given distinct
// per-stream keys, an update to one stream's subscription does not fire a
// SotW response into a tracked watch belonging to a different stream.
//
// Before the per-stream-key fix the SnapshotCache held one snapshot per
// `node.id`, so a tracked watch on stream A could be dispatched into when
// stream B's `SetSnapshot` rewrote that one snapshot, producing a SotW LDS
// response that dropped one of stream A's subscribed listeners. With each
// stream keyed independently, stream B's `SetSnapshot` lands on a different
// cache entry and stream A's watch is never touched.
func TestDistributor_DoesNotDispatchAcrossStreams(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sc := cache.NewSnapshotCache(true, cache.IDHash{}, testLogger{})
	sd := NewServiceDistributor(sc)

	services := []*entity.Service{
		newService("origin-service-1"),
		newService("origin-service-2"),
	}

	const streamKeyA = "stream:1"
	const streamKeyB = "stream:2"

	if err := sd.RegisterClient(ctx, streamKeyA, []string{"origin-service-1", "origin-service-2"}); err != nil {
		t.Fatalf("RegisterClient (stream A): %v", err)
	}
	if err := sd.DistributeServicesToClient(ctx, services, streamKeyA, []string{"origin-service-1", "origin-service-2"}); err != nil {
		t.Fatalf("DistributeServicesToClient (stream A): %v", err)
	}
	snap, err := sc.GetSnapshot(streamKeyA)
	if err != nil {
		t.Fatalf("GetSnapshot(%q): %v", streamKeyA, err)
	}
	versionA := snap.GetVersion(resource.ListenerType)

	// Simulate the post-ACK state where SnapshotCache is tracking stream A's
	// watch (createResponse returns nil because the snapshot version matches
	// and nothing has changed). With a different per-stream key, stream B's
	// distribution lands on a different cache entry and must not fire into
	// this watch.
	streamARespCh := make(chan cache.Response, 1)
	streamAReq := &discovery.DiscoveryRequest{
		Node:          &core.Node{Id: streamKeyA},
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

	if err := sd.RegisterClient(ctx, streamKeyB, []string{"origin-service-1"}); err != nil {
		t.Fatalf("RegisterClient (stream B): %v", err)
	}
	if err := sd.DistributeServicesToClient(ctx, services, streamKeyB, []string{"origin-service-1"}); err != nil {
		t.Fatalf("DistributeServicesToClient (stream B): %v", err)
	}

	select {
	case resp := <-streamARespCh:
		dr, err := resp.GetDiscoveryResponse()
		if err != nil {
			t.Fatalf("decode stream A response: %v", err)
		}
		t.Errorf("stream A's tracked watch was dispatched into when stream B's distribution updated a DIFFERENT cache entry: "+
			"got %d resource(s) %v at version=%q. "+
			"Per-stream cache keys should keep distinct streams' watches independent; receiving a response here means the cache key disambiguation has regressed.",
			len(dr.GetResources()),
			listenerNamesFromResources(t, dr.GetResources()),
			dr.GetVersionInfo())
	case <-time.After(500 * time.Millisecond):
		// expected
	}
}

// TestDistributor_UnregisterClient verifies that UnregisterClient removes
// the stream's subscription state and clears its snapshot, which is the
// teardown path invoked from OnStreamClosed when an ADS stream ends.
func TestDistributor_UnregisterClient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sc := cache.NewSnapshotCache(true, cache.IDHash{}, testLogger{})
	sd := NewServiceDistributor(sc)

	services := []*entity.Service{newService("origin-service-1")}

	const streamKey = "stream:1"

	if err := sd.RegisterClient(ctx, streamKey, []string{"origin-service-1"}); err != nil {
		t.Fatalf("RegisterClient: %v", err)
	}
	if err := sd.RegisterClustersToClient(ctx, streamKey, []string{"origin-service-1.example.com"}); err != nil {
		t.Fatalf("RegisterClustersToClient: %v", err)
	}
	if err := sd.DistributeServicesToClient(ctx, services, streamKey, []string{"origin-service-1"}); err != nil {
		t.Fatalf("DistributeServicesToClient: %v", err)
	}
	if _, err := sc.GetSnapshot(streamKey); err != nil {
		t.Fatalf("precondition: snapshot should exist before UnregisterClient: %v", err)
	}

	if err := sd.UnregisterClient(ctx, streamKey); err != nil {
		t.Fatalf("UnregisterClient: %v", err)
	}

	sd.clientListenersMu.RLock()
	_, listenersStillRegistered := sd.clientListenersMu.clientRequestedListeners[streamKey]
	sd.clientListenersMu.RUnlock()
	if listenersStillRegistered {
		t.Errorf("UnregisterClient did not remove the listener subscription for %q", streamKey)
	}

	sd.clientClustersMu.RLock()
	_, clustersStillRegistered := sd.clientClustersMu.clientRequestedClusters[streamKey]
	sd.clientClustersMu.RUnlock()
	if clustersStillRegistered {
		t.Errorf("UnregisterClient did not remove the cluster subscription for %q", streamKey)
	}

	if _, err := sc.GetSnapshot(streamKey); err == nil {
		t.Errorf("UnregisterClient did not clear the snapshot for %q", streamKey)
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
