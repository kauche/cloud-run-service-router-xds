package e2etest

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/anypb"
)

var client discovery.AggregatedDiscoveryServiceClient

var cmpoptSortListeners = cmpopts.SortSlices(func(x, y *listener.Listener) bool {
	return strings.Compare(x.Name, y.Name) < 0
})

var cmpoptSortClusters = cmpopts.SortSlices(func(x, y *cluster.Cluster) bool {
	return strings.Compare(x.Name, y.Name) < 0
})

func TestMain(m *testing.M) {
	os.Exit(func() int {
		ctx := context.Background()

		// TODO: target
		cc, err := grpc.DialContext(ctx, "localhost:11000", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to dial to the control-plane: %s", err)

			return 1
		}
		defer cc.Close()

		client = discovery.NewAggregatedDiscoveryServiceClient(cc)

		return m.Run()
	}())
}

func TestE2E_ListSpecificListeners(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	strem, err := client.StreamAggregatedResources(ctx)
	if err != nil {
		t.Errorf("failed to create a stream: %s", err)
		return
	}

	if err = strem.Send(&discovery.DiscoveryRequest{
		TypeUrl: "type.googleapis.com/envoy.config.listener.v3.Listener",
		Node: &core.Node{
			Id: "test-1",
		},
		ResourceNames: []string{"origin-service-1"},
	}); err != nil {
		t.Errorf("failed to send a request: %s", err)
		return
	}

	lres, err := strem.Recv()
	if err != nil {
		t.Errorf("failed to receive a response: %s", err)
		return
	}

	lgot := make([]*listener.Listener, len(lres.Resources))
	unmarshalOptions := proto.UnmarshalOptions{}
	for i, resource := range lres.Resources {
		lgot[i] = new(listener.Listener)
		if err = anypb.UnmarshalTo(resource, lgot[i], unmarshalOptions); err != nil {
			t.Errorf("failed to unmarshal xds response: %s", err)
			return
		}
	}

	lis1, err := newListener(t, "origin-service-1", []string{"route-service-1"})
	if err != nil {
		t.Errorf("failed to create a listener: %s", err)
		return
	}
	want := []*listener.Listener{
		lis1,
	}

	if diff := cmp.Diff(lgot, want, protocmp.Transform(), cmpoptSortListeners); diff != "" {
		t.Errorf("\n(-got, +want)\n%s", diff)
		return
	}

	if err = strem.Send(&discovery.DiscoveryRequest{
		TypeUrl: "type.googleapis.com/envoy.config.cluster.v3.Cluster",
		Node: &core.Node{
			Id: "test-1",
		},
		ResourceNames: []string{"origin-service-1-test-an.a.run.app", "route-service-1-test-an.a.run.app"},
	}); err != nil {
		t.Errorf("failed to send a request: %s", err)
		return
	}

	cres, err := strem.Recv()
	if err != nil {
		t.Errorf("failed to receive a response: %s", err)
		return
	}

	cgot := make([]*cluster.Cluster, len(cres.Resources))
	for i, resource := range cres.Resources {
		cgot[i] = new(cluster.Cluster)
		if err = anypb.UnmarshalTo(resource, cgot[i], unmarshalOptions); err != nil {
			t.Errorf("failed to unmarshal xds response: %s", err)
			return
		}
	}

	cwant := []*cluster.Cluster{
		newCluster(t, "origin-service-1-test-an.a.run.app"),
		newCluster(t, "route-service-1-test-an.a.run.app"),
	}

	if diff := cmp.Diff(cgot, cwant, protocmp.Transform(), cmpoptSortClusters); diff != "" {
		t.Errorf("\n(-got, +want)\n%s", diff)
		return
	}
}

func TestE2E_ListMultipleListerns(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	strem, err := client.StreamAggregatedResources(ctx)
	if err != nil {
		t.Errorf("failed to create a stream: %s", err)
		return
	}

	if err = strem.Send(&discovery.DiscoveryRequest{
		TypeUrl: "type.googleapis.com/envoy.config.listener.v3.Listener",
		Node: &core.Node{
			Id: "test-2",
		},
		ResourceNames: []string{"origin-service-1", "origin-service-2"},
	}); err != nil {
		t.Errorf("failed to send a request: %s", err)
		return
	}

	lres, err := strem.Recv()
	if err != nil {
		t.Errorf("failed to receive a response: %s", err)
		return
	}

	lgot := make([]*listener.Listener, len(lres.Resources))
	unmarshalOptions := proto.UnmarshalOptions{}
	for i, resource := range lres.Resources {
		lgot[i] = new(listener.Listener)
		if err = anypb.UnmarshalTo(resource, lgot[i], unmarshalOptions); err != nil {
			t.Errorf("failed to unmarshal xds response: %s", err)
			return
		}
	}

	l1, err := newListener(t, "origin-service-1", []string{"route-service-1"})
	if err != nil {
		t.Errorf("failed to create a listener: %s", err)
		return
	}

	l2, err := newListener(t, "origin-service-2", []string{"route-service-2", "route-service-3"})
	if err != nil {
		t.Errorf("failed to create a listener: %s", err)
		return
	}

	lwant := []*listener.Listener{
		l1,
		l2,
	}

	if diff := cmp.Diff(lgot, lwant, protocmp.Transform(), cmpoptSortListeners); diff != "" {
		t.Errorf("\n(-got, +want)\n%s", diff)
		return
	}

	if err = strem.Send(&discovery.DiscoveryRequest{
		TypeUrl: "type.googleapis.com/envoy.config.cluster.v3.Cluster",
		Node: &core.Node{
			Id: "test-2",
		},
		ResourceNames: []string{
			"origin-service-1-test-an.a.run.app",
			"origin-service-2-test-an.a.run.app",
			"route-service-1-test-an.a.run.app",
			"route-service-2-test-an.a.run.app",
			"route-service-3-test-an.a.run.app",
		},
	}); err != nil {
		t.Errorf("failed to send a request: %s", err)
		return
	}

	cres, err := strem.Recv()
	if err != nil {
		t.Errorf("failed to receive a response: %s", err)
		return
	}

	cgot := make([]*cluster.Cluster, len(cres.Resources))
	for i, resource := range cres.Resources {
		cgot[i] = new(cluster.Cluster)
		if err = anypb.UnmarshalTo(resource, cgot[i], unmarshalOptions); err != nil {
			t.Errorf("failed to unmarshal xds response: %s", err)
			return
		}
	}

	cwant := []*cluster.Cluster{
		newCluster(t, "origin-service-1-test-an.a.run.app"),
		newCluster(t, "origin-service-2-test-an.a.run.app"),
		newCluster(t, "route-service-1-test-an.a.run.app"),
		newCluster(t, "route-service-2-test-an.a.run.app"),
		newCluster(t, "route-service-3-test-an.a.run.app"),
	}

	if diff := cmp.Diff(cgot, cwant, protocmp.Transform(), cmpoptSortClusters); diff != "" {
		t.Errorf("\n(-got, +want)\n%s", diff)
		return
	}
}

func TestE2E_ListAllResources(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	strem, err := client.StreamAggregatedResources(ctx)
	if err != nil {
		t.Errorf("failed to create a stream: %s", err)
		return
	}

	if err = strem.Send(&discovery.DiscoveryRequest{
		TypeUrl: "type.googleapis.com/envoy.config.listener.v3.Listener",
		Node: &core.Node{
			Id: "test-3",
		},
		ResourceNames: []string{},
	}); err != nil {
		t.Errorf("failed to send a request: %s", err)
		return
	}

	lres, err := strem.Recv()
	if err != nil {
		t.Errorf("failed to receive a response: %s", err)
		return
	}

	lgot := make([]*listener.Listener, len(lres.Resources))
	unmarshalOptions := proto.UnmarshalOptions{}
	for i, resource := range lres.Resources {
		lgot[i] = new(listener.Listener)
		if err = anypb.UnmarshalTo(resource, lgot[i], unmarshalOptions); err != nil {
			t.Errorf("failed to unmarshal xds response: %s", err)
			return
		}
	}

	l1, err := newListener(t, "origin-service-1", []string{"route-service-1"})
	if err != nil {
		t.Errorf("failed to create a listener: %s", err)
		return
	}

	l2, err := newListener(t, "origin-service-2", []string{"route-service-2", "route-service-3"})
	if err != nil {
		t.Errorf("failed to create a listener: %s", err)
		return
	}

	l3, err := newListener(t, "origin-service-without-route", []string{})
	if err != nil {
		t.Errorf("failed to create a listener: %s", err)
		return
	}

	lwant := []*listener.Listener{
		l1,
		l2,
		l3,
	}

	if diff := cmp.Diff(lgot, lwant, protocmp.Transform(), cmpoptSortListeners); diff != "" {
		t.Errorf("\n(-got, +want)\n%s", diff)
		return
	}

	if err = strem.Send(&discovery.DiscoveryRequest{
		TypeUrl: "type.googleapis.com/envoy.config.cluster.v3.Cluster",
		Node: &core.Node{
			Id: "test-3",
		},
		ResourceNames: []string{
			"origin-service-1-test-an.a.run.app",
			"origin-service-2-test-an.a.run.app",
			"origin-service-without-route-test-an.a.run.app",
			"route-service-1-test-an.a.run.app",
			"route-service-2-test-an.a.run.app",
			"route-service-3-test-an.a.run.app",
		},
	}); err != nil {
		t.Errorf("failed to send a request: %s", err)
		return
	}

	cres, err := strem.Recv()
	if err != nil {
		t.Errorf("failed to receive a response: %s", err)
		return
	}

	cgot := make([]*cluster.Cluster, len(cres.Resources))
	for i, resource := range cres.Resources {
		cgot[i] = new(cluster.Cluster)
		if err = anypb.UnmarshalTo(resource, cgot[i], unmarshalOptions); err != nil {
			t.Errorf("failed to unmarshal xds response: %s", err)
			return
		}
	}

	cwant := []*cluster.Cluster{
		newCluster(t, "origin-service-1-test-an.a.run.app"),
		newCluster(t, "origin-service-2-test-an.a.run.app"),
		newCluster(t, "origin-service-without-route-test-an.a.run.app"),
		newCluster(t, "route-service-1-test-an.a.run.app"),
		newCluster(t, "route-service-2-test-an.a.run.app"),
		newCluster(t, "route-service-3-test-an.a.run.app"),
	}

	if diff := cmp.Diff(cgot, cwant, protocmp.Transform(), cmpoptSortClusters); diff != "" {
		t.Errorf("\n(-got, +want)\n%s", diff)
		return
	}
}

func TestE2E_ListMultipleClusters(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	strem, err := client.StreamAggregatedResources(ctx)
	if err != nil {
		t.Errorf("failed to create a stream: %s", err)
		return
	}

	if err = strem.Send(&discovery.DiscoveryRequest{
		TypeUrl: "type.googleapis.com/envoy.config.listener.v3.Listener",
		Node: &core.Node{
			Id: "test-4",
		},
		ResourceNames: []string{"origin-service-1", "origin-service-2"},
	}); err != nil {
		t.Errorf("failed to send a request: %s", err)
		return
	}

	lres, err := strem.Recv()
	if err != nil {
		t.Errorf("failed to receive a response: %s", err)
		return
	}

	lgot := make([]*listener.Listener, len(lres.Resources))
	unmarshalOptions := proto.UnmarshalOptions{}
	for i, resource := range lres.Resources {
		lgot[i] = new(listener.Listener)
		if err = anypb.UnmarshalTo(resource, lgot[i], unmarshalOptions); err != nil {
			t.Errorf("failed to unmarshal xds response: %s", err)
			return
		}
	}

	l1, err := newListener(t, "origin-service-1", []string{"route-service-1"})
	if err != nil {
		t.Errorf("failed to create a listener: %s", err)
		return
	}

	l2, err := newListener(t, "origin-service-2", []string{"route-service-2", "route-service-3"})
	if err != nil {
		t.Errorf("failed to create a listener: %s", err)
		return
	}

	lwant := []*listener.Listener{
		l1,
		l2,
	}

	if diff := cmp.Diff(lgot, lwant, protocmp.Transform(), cmpoptSortListeners); diff != "" {
		t.Errorf("\n(-got, +want)\n%s", diff)
		return
	}

	if err = strem.Send(&discovery.DiscoveryRequest{
		TypeUrl: "type.googleapis.com/envoy.config.cluster.v3.Cluster",
		Node: &core.Node{
			Id: "test-4",
		},
		ResourceNames: []string{
			"origin-service-1-test-an.a.run.app",
			"route-service-1-test-an.a.run.app",
		},
	}); err != nil {
		t.Errorf("failed to send a request: %s", err)
		return
	}

	cres, err := strem.Recv()
	if err != nil {
		t.Errorf("failed to receive a response: %s", err)
		return
	}

	cgot1 := make([]*cluster.Cluster, len(cres.Resources))
	for i, resource := range cres.Resources {
		cgot1[i] = new(cluster.Cluster)
		if err = anypb.UnmarshalTo(resource, cgot1[i], unmarshalOptions); err != nil {
			t.Errorf("failed to unmarshal xds response: %s", err)
			return
		}
	}

	cwant1 := []*cluster.Cluster{
		newCluster(t, "origin-service-1-test-an.a.run.app"),
		newCluster(t, "route-service-1-test-an.a.run.app"),
	}

	if diff := cmp.Diff(cgot1, cwant1, protocmp.Transform(), cmpoptSortClusters); diff != "" {
		t.Errorf("\n(-got, +want)\n%s", diff)
		return
	}

	if err = strem.Send(&discovery.DiscoveryRequest{
		TypeUrl: "type.googleapis.com/envoy.config.cluster.v3.Cluster",
		Node: &core.Node{
			Id: "test-4",
		},
		VersionInfo:   cres.VersionInfo,
		ResponseNonce: cres.Nonce,
		ResourceNames: []string{
			"origin-service-2-test-an.a.run.app",
			"route-service-2-test-an.a.run.app",
			"route-service-3-test-an.a.run.app",
		},
	}); err != nil {
		t.Errorf("failed to send a request: %s", err)
		return
	}

	cres2, err := strem.Recv()
	if err != nil {
		t.Errorf("failed to receive a response: %s", err)
		return
	}

	cgot2 := make([]*cluster.Cluster, len(cres2.Resources))
	for i, resource := range cres2.Resources {
		cgot2[i] = new(cluster.Cluster)
		if err = anypb.UnmarshalTo(resource, cgot2[i], unmarshalOptions); err != nil {
			t.Errorf("failed to unmarshal xds response: %s", err)
			return
		}
	}

	cwant2 := []*cluster.Cluster{
		newCluster(t, "origin-service-2-test-an.a.run.app"),
		newCluster(t, "route-service-2-test-an.a.run.app"),
		newCluster(t, "route-service-3-test-an.a.run.app"),
	}

	if diff := cmp.Diff(cgot2, cwant2, protocmp.Transform(), cmpoptSortClusters); diff != "" {
		t.Errorf("\n(-got, +want)\n%s", diff)
		return
	}
}

// TestE2E_MultiStreamSameNodeID is the end-to-end counterpart of
// TestMultiStreamSameNodeID_TriggersResourceRemoved in
// internal/driver/distributor/xds. It reproduces the multi-stream
// subscription collision over a real ADS gRPC stream.
//
// Two ADS streams to the same control plane carry the same `node.id` — which
// is the default condition for any Go process that builds more than one
// `xds:///...` channel since [grpc-go v1.66 (PR #7347)] made each channel its
// own xDS client / ADS stream while keeping the bootstrap node.id
// per-process.
//
// Stream A subscribes (via LDS) to BOTH origin-service-1 and
// origin-service-2 and ACKs the initial response so the SnapshotCache starts
// tracking its watch. Stream B then opens against the same node.id and
// subscribes only to origin-service-1. The control plane rewrites the
// single snapshot keyed by that node.id to only include origin-service-1,
// and dispatches into Stream A's tracked watch a SotW LDS response that
// silently drops origin-service-2. On a real grpc-go client this surfaces as
// `xds: resource "origin-service-2" of type "ListenerResource" has been
// removed`, the xDS resolver pushes an erroring config selector, the channel
// transitions to TRANSIENT_FAILURE, and the next RPC fails.
//
// While the bug is present this test FAILS. Once subscriptions from distinct
// streams on the same node.id are disambiguated (per-stream cache key, or
// merged subscriptions across streams under one node.id) the cache no longer
// dispatches into Stream A from Stream B's distribution and the test passes
// via the read timeout.
//
// [grpc-go v1.66 (PR #7347)]: https://github.com/grpc/grpc-go/pull/7347
func TestE2E_MultiStreamSameNodeID(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	const nodeID = "test-multi-stream-collision"

	// --- Stream A: subscribe to both listeners. ---
	streamA, err := client.StreamAggregatedResources(ctx)
	if err != nil {
		t.Fatalf("create stream A: %s", err)
	}

	if err := streamA.Send(&discovery.DiscoveryRequest{
		TypeUrl:       "type.googleapis.com/envoy.config.listener.v3.Listener",
		Node:          &core.Node{Id: nodeID},
		ResourceNames: []string{"origin-service-1", "origin-service-2"},
	}); err != nil {
		t.Fatalf("stream A initial LDS send: %s", err)
	}

	initialResp, err := streamA.Recv()
	if err != nil {
		t.Fatalf("stream A initial LDS recv: %s", err)
	}

	initialNames := listenerNamesFromAny(t, initialResp.GetResources())
	if !slices.Contains(initialNames, "origin-service-1") || !slices.Contains(initialNames, "origin-service-2") {
		t.Fatalf("precondition violated: stream A's initial LDS response should contain both listeners; got %v", initialNames)
	}

	// Stream A: ACK. This second request with the just-received VersionInfo
	// makes the SnapshotCache start tracking stream A's watch, which is the
	// state required for a subsequent SetSnapshot to dispatch into it.
	if err := streamA.Send(&discovery.DiscoveryRequest{
		TypeUrl:       "type.googleapis.com/envoy.config.listener.v3.Listener",
		Node:          &core.Node{Id: nodeID},
		ResourceNames: []string{"origin-service-1", "origin-service-2"},
		VersionInfo:   initialResp.GetVersionInfo(),
		ResponseNonce: initialResp.GetNonce(),
	}); err != nil {
		t.Fatalf("stream A LDS ACK send: %s", err)
	}

	// Allow the control plane to process the ACK and add the tracked watch
	// before stream B fires. We can't synchronise on this directly over the
	// wire (the ACK does not produce a response), so we give it a generous
	// grace period.
	time.Sleep(500 * time.Millisecond)

	// --- Stream B: open a SECOND ADS stream with the SAME node.id and
	// subscribe to a strict subset of stream A's resources. This is the
	// collision that, on the same control-plane process, also happens between
	// two `xds:///...` grpc.Channels in the same Go process under grpc-go
	// v1.66+. ---
	streamB, err := client.StreamAggregatedResources(ctx)
	if err != nil {
		t.Fatalf("create stream B: %s", err)
	}

	if err := streamB.Send(&discovery.DiscoveryRequest{
		TypeUrl:       "type.googleapis.com/envoy.config.listener.v3.Listener",
		Node:          &core.Node{Id: nodeID},
		ResourceNames: []string{"origin-service-1"},
	}); err != nil {
		t.Fatalf("stream B initial LDS send: %s", err)
	}

	if _, err := streamB.Recv(); err != nil {
		t.Fatalf("stream B initial LDS recv: %s", err)
	}

	// --- Stream A: read with a timeout. With the bug present, the cache
	// has just dispatched into stream A's tracked watch a SotW LDS response
	// without origin-service-2; we Recv it and inspect it. ---
	type recvResult struct {
		resp *discovery.DiscoveryResponse
		err  error
	}
	respCh := make(chan recvResult, 1)
	go func() {
		resp, err := streamA.Recv()
		respCh <- recvResult{resp: resp, err: err}
	}()

	// Expected successful end state: stream A's tracked watch is never
	// dispatched into and the read times out. This is the post-fix state
	// produced by per-stream cache key disambiguation: stream B's
	// `SetSnapshot` lands on a different cache entry from stream A's
	// tracked watch, so stream A is never touched.
	//
	// Any response received here means the cache dispatched into stream
	// A's watch from stream B's distribution, which is the bug — even if
	// the response happens to contain every listener stream A subscribed
	// to (which would happen under a subscription-union fix), because the
	// control plane has chosen the per-stream-key path.
	select {
	case got := <-respCh:
		if got.err != nil {
			t.Fatalf("stream A second LDS recv: %s", got.err)
		}
		names := listenerNamesFromAny(t, got.resp.GetResources())
		t.Errorf("multi-stream collision: stream A's tracked LDS watch was dispatched into from stream B's distribution. "+
			"stream A subscribed to %v over an ADS stream with node.id=%q; "+
			"stream B then subscribed to %v over a second ADS stream with the SAME node.id; "+
			"stream A subsequently received a SotW LDS DiscoveryResponse: got %d resource(s) %v at version=%q. "+
			"Per-stream-key disambiguation expects stream A's watch to remain silent because stream B's SetSnapshot should land on a different cache entry; "+
			"a phantom resource removal (response missing one of stream A's subscribed listeners) on a real grpc-go client surfaces as `xds: resource ... has been removed`, "+
			"pushes an erroring config selector through the xDS resolver, transitions the channel to TRANSIENT_FAILURE, and fails the next RPC.",
			[]string{"origin-service-1", "origin-service-2"},
			nodeID,
			[]string{"origin-service-1"},
			len(got.resp.GetResources()), names, got.resp.GetVersionInfo())
	case <-time.After(3 * time.Second):
		// Stream A's tracked watch was not dispatched into. The absence
		// of a response IS the assertion.
	}
}

func listenerNamesFromAny(t *testing.T, resources []*anypb.Any) []string {
	t.Helper()
	out := make([]string, 0, len(resources))
	unmarshalOptions := proto.UnmarshalOptions{}
	for _, r := range resources {
		lis := &listener.Listener{}
		if err := anypb.UnmarshalTo(r, lis, unmarshalOptions); err != nil {
			t.Fatalf("unmarshal listener: %s", err)
		}
		out = append(out, lis.GetName())
	}
	return out
}

func newListener(t *testing.T, name string, routes []string) (*listener.Listener, error) {
	t.Helper()

	rs := make([]*route.Route, len(routes)+1)

	for i, r := range routes {
		rs[i] = newRoute(t, r, name)
	}

	rs[len(routes)] = newRoute(t, name, name)

	hc := &hcm.HttpConnectionManager{
		HttpFilters: []*hcm.HttpFilter{
			{
				Name: "envoy.filters.http.router",
				ConfigType: &hcm.HttpFilter_TypedConfig{
					TypedConfig: &anypb.Any{
						TypeUrl: "type.googleapis.com/envoy.extensions.filters.http.router.v3.Router",
					},
				},
			},
		},
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &route.RouteConfiguration{
				VirtualHosts: []*route.VirtualHost{
					{
						Name:    name,
						Domains: []string{name},
						Routes:  rs,
					},
				},
			},
		},
	}

	hcb, err := proto.Marshal(hc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal hcb: %w", err)
	}

	return &listener.Listener{
		Name: name,
		ApiListener: &listener.ApiListener{
			ApiListener: &anypb.Any{
				TypeUrl: "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager",
				Value:   hcb,
			},
		},
	}, nil
}

func newRoute(t *testing.T, name, originServiceName string) *route.Route {
	t.Helper()

	var headers []*route.HeaderMatcher
	if name != originServiceName {
		headers = []*route.HeaderMatcher{
			{
				Name: fmt.Sprintf("cloud-run-service-router-%s", originServiceName),
				HeaderMatchSpecifier: &route.HeaderMatcher_ExactMatch{
					ExactMatch: name,
				},
			},
		}
	}

	action := &route.Route_Route{
		Route: &route.RouteAction{
			ClusterSpecifier: &route.RouteAction_Cluster{
				Cluster: fmt.Sprintf("%s-test-an.a.run.app", name),
			},
			Timeout: &duration.Duration{Seconds: 10},
		},
	}

	action.Route.HostRewriteSpecifier = &route.RouteAction_AutoHostRewrite{
		AutoHostRewrite: &wrappers.BoolValue{
			Value: true,
		},
	}

	return &route.Route{
		Name: name,
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: "/",
			},
			Headers: headers,
		},
		Action: action,
	}
}

func newCluster(t *testing.T, name string) *cluster.Cluster {
	t.Helper()

	return &cluster.Cluster{
		Name: name,
		ClusterDiscoveryType: &cluster.Cluster_Type{
			Type: cluster.Cluster_LOGICAL_DNS,
		},
		LbPolicy: cluster.Cluster_ROUND_ROBIN,
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: name,
			Endpoints: []*endpoint.LocalityLbEndpoints{
				{
					LbEndpoints: []*endpoint.LbEndpoint{
						{
							HostIdentifier: &endpoint.LbEndpoint_Endpoint{
								Endpoint: &endpoint.Endpoint{
									Hostname: name,
									Address: &core.Address{
										Address: &core.Address_SocketAddress{
											SocketAddress: &core.SocketAddress{
												Address: name,
												PortSpecifier: &core.SocketAddress_PortValue{
													PortValue: 443,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
