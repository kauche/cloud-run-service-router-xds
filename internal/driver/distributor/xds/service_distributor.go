package xds

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	duration "github.com/golang/protobuf/ptypes/duration"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/kauche/cloud-run-service-router-xds/internal/domain/distributor"
	"github.com/kauche/cloud-run-service-router-xds/internal/domain/entity"
)

var _ distributor.ServiceDistributor = (*ServiceDistributor)(nil)

type ServiceDistributor struct {
	snapshotCache cache.SnapshotCache

	clientListenersMu struct {
		sync.RWMutex
		clientRequestedListeners map[string][]string
	}

	clientClustersMu struct {
		sync.RWMutex
		clientRequestedClusters map[string][]string
	}
}

func NewServiceDistributor(sc cache.SnapshotCache) *ServiceDistributor {
	d := &ServiceDistributor{
		snapshotCache: sc,
	}

	d.clientListenersMu.clientRequestedListeners = make(map[string][]string)
	d.clientClustersMu.clientRequestedClusters = make(map[string][]string)

	return d
}

func (d *ServiceDistributor) DistributeServices(ctx context.Context, services []*entity.Service) error {
	d.clientListenersMu.RLock()
	defer d.clientListenersMu.RUnlock()
	for client, resourceNames := range d.clientListenersMu.clientRequestedListeners {
		// TODO: should call concurrently
		if err := d.DistributeServicesToClient(ctx, services, client, resourceNames); err != nil {
			return fmt.Errorf("failed to distribute Listenres to the client:%q : %w", client, err)
		}
	}

	d.clientClustersMu.RLock()
	defer d.clientClustersMu.RUnlock()
	for client, resourceNames := range d.clientClustersMu.clientRequestedClusters {
		// TODO: should call concurrently
		if err := d.DistributeClustersToClient(ctx, services, client, resourceNames); err != nil {
			return fmt.Errorf("failed to distribute Clusters to the client:%q : %w", client, err)
		}
	}

	return nil
}

func (d *ServiceDistributor) DistributeServicesToClient(ctx context.Context, services []*entity.Service, client string, resouceNames []string) error {
	listeners, version, err := generateListeners(services, resouceNames)
	if err != nil {
		return fmt.Errorf("failed to generate Listeners: %w", err)
	}

	out := &cache.Snapshot{}

	osc, err := d.snapshotCache.GetSnapshot(client)
	if err == nil {
		cv := osc.GetVersion(resource.ClusterType)

		var clusters []types.Resource
		for _, v := range osc.GetResources(resource.ClusterType) {
			clusters = append(clusters, v)
		}

		out.Resources[cache.GetResponseType(resource.ClusterType)] = cache.NewResources(cv, clusters)
	}

	out.Resources[cache.GetResponseType(resource.ListenerType)] = cache.NewResources(version, listeners)

	if err := d.snapshotCache.SetSnapshot(ctx, client, out); err != nil {
		return fmt.Errorf("failed to create a snapshot cache to the client `%s`: %w", client, err)
	}

	return nil
}

func (d *ServiceDistributor) DistributeClustersToClient(ctx context.Context, services []*entity.Service, client string, resourceNames []string) error {
	clusters, version, err := generateClusters(services, resourceNames)
	if err != nil {
		return fmt.Errorf("failed to generate Clusters: %w", err)
	}

	out := &cache.Snapshot{}

	osc, err := d.snapshotCache.GetSnapshot(client)
	if err == nil {
		lv := osc.GetVersion(resource.ListenerType)

		var listeners []types.Resource
		for _, v := range osc.GetResources(resource.ListenerType) {
			listeners = append(listeners, v)
		}

		out.Resources[cache.GetResponseType(resource.ListenerType)] = cache.NewResources(lv, listeners)
	}

	out.Resources[cache.GetResponseType(resource.ClusterType)] = cache.NewResources(version, clusters)

	if err := d.snapshotCache.SetSnapshot(ctx, client, out); err != nil {
		return fmt.Errorf("failed to create a snapshot cache to the client `%s`: %w", client, err)
	}

	return nil
}

func (d *ServiceDistributor) RegisterClient(ctx context.Context, client string, serviceNames []string) error {
	d.clientListenersMu.Lock()
	d.clientListenersMu.clientRequestedListeners[client] = serviceNames
	defer d.clientListenersMu.Unlock()

	return nil
}

func (d *ServiceDistributor) RegisterClustersToClient(ctx context.Context, client string, serviceNames []string) error {
	d.clientClustersMu.Lock()
	d.clientClustersMu.clientRequestedClusters[client] = serviceNames
	defer d.clientClustersMu.Unlock()

	return nil
}

func generateListeners(services []*entity.Service, requestedNames []string) ([]types.Resource, string, error) {
	if len(services) == 0 {
		return []types.Resource{}, "", nil
	}

	var listeners []types.Resource

	shoudDistributeAll := len(requestedNames) == 0

	names := make(map[string]struct{})
	for _, name := range requestedNames {
		names[name] = struct{}{}
	}

	sort.SliceStable(services, func(i, j int) bool {
		return strings.Compare(services[i].Name, services[j].Name) < 0
	})

	versionHash := sha256.New()

	for _, service := range services {
		_, ok := names[service.Name]
		if !shoudDistributeAll && !ok {
			continue
		}

		var routes []*route.Route

		i := 0
		for _, r := range service.Routes {
			routes = append(routes, &route.Route{
				Name: r.Name,
				Match: &route.RouteMatch{
					PathSpecifier: &route.RouteMatch_Prefix{
						Prefix: "/",
					},
					Headers: []*route.HeaderMatcher{
						{
							Name: fmt.Sprintf("cloud-run-service-router-%s", service.Name), // TODO: This header prefix should be configurable.
							HeaderMatchSpecifier: &route.HeaderMatcher_ExactMatch{
								ExactMatch: r.Name,
							},
						},
					},
				},
				Action: &route.Route_Route{
					Route: &route.RouteAction{
						ClusterSpecifier: &route.RouteAction_Cluster{
							Cluster: r.Host,
						},
						Timeout: &duration.Duration{Seconds: 10}, // TODO: This timeout duration should be configurable.
					},
				},
			})

			i++
		}

		sort.SliceStable(routes, func(x, y int) bool {
			return strings.Compare(routes[x].Name, routes[y].Name) < 0
		})

		routes = append(routes, &route.Route{
			Name: service.Name,
			Match: &route.RouteMatch{
				PathSpecifier: &route.RouteMatch_Prefix{
					Prefix: "/",
				},
			},
			Action: &route.Route_Route{
				Route: &route.RouteAction{
					ClusterSpecifier: &route.RouteAction_Cluster{
						Cluster: service.DefaultRoute.Host,
					},
					Timeout: &duration.Duration{Seconds: 10}, // TODO: This timeout duration should be configurable.
				},
			},
		})

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
							Name:    service.Name,
							Domains: []string{service.Name},
							Routes:  routes,
						},
					},
				},
			},
		}

		hcb, err := proto.Marshal(hc)
		if err != nil {
			return nil, "", fmt.Errorf("failed to marshal a HttpConnectionManager protobuf: %w", err)
		}

		lis := &listener.Listener{
			Name: service.Name,
			ApiListener: &listener.ApiListener{
				ApiListener: &anypb.Any{
					TypeUrl: "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager",
					Value:   hcb,
				},
			},
		}

		listeners = append(listeners, lis)
		_, err = io.WriteString(versionHash, lis.Name)
		if err != nil {
			return nil, "", fmt.Errorf("failed to write string to version has for listner/%q: %w", lis.Name, err)
		}
	}

	return listeners, fmt.Sprintf("%x", versionHash.Sum(nil)), nil
}

func generateClusters(services []*entity.Service, requestedNames []string) ([]types.Resource, string, error) {
	if len(services) == 0 {
		return []types.Resource{}, "", nil
	}

	var clusters []types.Resource

	shoudDistributeAll := len(requestedNames) == 0

	names := make(map[string]struct{})
	for _, name := range requestedNames {
		names[name] = struct{}{}
	}

	sort.SliceStable(services, func(i, j int) bool {
		return strings.Compare(services[i].Name, services[j].Name) < 0
	})

	versionHash := sha256.New()

	for _, service := range services {
		var routes []*entity.Route
		for _, r := range service.Routes {
			_, ok := names[r.Host]
			if !shoudDistributeAll && !ok {
				continue
			}
			routes = append(routes, r)
		}

		_, ok := names[service.DefaultRoute.Host]
		if shoudDistributeAll || ok {
			routes = append(routes, service.DefaultRoute)
		}

		sort.SliceStable(routes, func(i, j int) bool {
			return strings.Compare(routes[i].Name, routes[j].Name) < 0
		})

		for _, r := range routes {
			clu := createCluster(r.Host)
			clusters = append(clusters, clu)

			_, err := io.WriteString(versionHash, clu.Name)
			if err != nil {
				return nil, "", fmt.Errorf("failed to write string to version has for cluster/%q: %w", clu.Name, err)
			}
		}
	}

	return clusters, fmt.Sprintf("%x", versionHash.Sum(nil)), nil
}

func createCluster(host string) *cluster.Cluster {
	return &cluster.Cluster{
		Name: host,
		ClusterDiscoveryType: &cluster.Cluster_Type{
			Type: cluster.Cluster_LOGICAL_DNS,
		},
		LbPolicy: cluster.Cluster_ROUND_ROBIN,
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: host,
			Endpoints: []*endpoint.LocalityLbEndpoints{
				{
					LbEndpoints: []*endpoint.LbEndpoint{
						{
							HostIdentifier: &endpoint.LbEndpoint_Endpoint{
								Endpoint: &endpoint.Endpoint{
									Address: &core.Address{
										Address: &core.Address_SocketAddress{
											SocketAddress: &core.SocketAddress{
												Address: host,
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
