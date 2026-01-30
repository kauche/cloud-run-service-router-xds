package e2etest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

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
