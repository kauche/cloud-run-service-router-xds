package xds

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"github.com/kauche/cloud-run-service-router-xds/internal/domain/distributor"
	"github.com/kauche/cloud-run-service-router-xds/internal/domain/entity"
)

var _ distributor.ServiceDistributor = (*ServiceDistributor)(nil)

type ServiceDistributor struct {
	snapshotCache cache.SnapshotCache

	clientsMu struct {
		sync.RWMutex
		// [client] -> [service name] -> [version]
		clients map[string]map[string]string
	}
}

func NewServiceDistributor(sc cache.SnapshotCache) *ServiceDistributor {
	d := &ServiceDistributor{
		snapshotCache: sc,
	}

	d.clientsMu.clients = make(map[string]map[string]string)

	return d
}

func (d *ServiceDistributor) DistributeServices(ctx context.Context, services []*entity.Service) error {
	d.clientsMu.RLock()
	clients := make([]string, 0, len(d.clientsMu.clients))
	for k := range d.clientsMu.clients {
		clients = append(clients, k)
	}
	d.clientsMu.RUnlock()

	for _, client := range clients {
		if err := d.DistributeServicesToClient(ctx, services, client); err != nil {
			return fmt.Errorf("failed to distribute services to the client %s: %w", client, err)
		}
	}

	return nil
}

func (d *ServiceDistributor) DistributeServicesToClient(ctx context.Context, services []*entity.Service, client string) error {
	d.clientsMu.RLock()
	defer d.clientsMu.RUnlock()

	clientRequestedServices, ok := d.clientsMu.clients[client]
	if !ok {
		return errors.New("the client is not registered")
	}

	shouldDistributeAllServices := len(clientRequestedServices) == 0

	var listeners []types.Resource
	shouldUpdateResourceVersion := len(services) == 0

	for _, service := range services {
		requestedListenerVersion, ok := clientRequestedServices[service.Name]
		if !ok && !shouldDistributeAllServices {
			continue
		}

		if !shouldDistributeAllServices && (requestedListenerVersion != service.Version) {
			shouldUpdateResourceVersion = true

			// TODO: should be defered?
			d.clientsMu.RUnlock()
			d.clientsMu.Lock()
			clientRequestedServices[service.Name] = service.Version
			d.clientsMu.Unlock()
			d.clientsMu.RLock()
		}

		if shouldDistributeAllServices {
			shouldUpdateResourceVersion = true
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
			return fmt.Errorf("failed to marshal a HttpConnectionManager protobuf: %w", err)
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
	}

	var version string
	if shouldUpdateResourceVersion {
		uid, err := uuid.NewRandom()
		if err != nil {
			return fmt.Errorf("failed to create a version for the snapshot cache: %w", err)
		}
		version = uid.String()
	} else {
		snapshot, err := d.snapshotCache.GetSnapshot(client)
		if err != nil {
			return fmt.Errorf("failed to get the cached snapshot for the client %s: %w", client, err)
		}
		version = snapshot.GetVersion(resource.ListenerType)
	}

	resources := map[resource.Type][]types.Resource{
		resource.ListenerType: listeners,
	}

	sc, err := cache.NewSnapshot(version, resources)
	if err != nil {
		return fmt.Errorf("failed to create a new snapshot: %w", err)
	}

	if err := d.snapshotCache.SetSnapshot(ctx, client, sc); err != nil {
		return fmt.Errorf("failed to create a snapshot cache to the client `%s`: %w", client, err)
	}

	return nil
}

func (d *ServiceDistributor) DistributeClustersToClient(ctx context.Context, services []*entity.Service, client string) error {
	d.clientsMu.RLock()
	defer d.clientsMu.RUnlock()

	clientRequestedServices, ok := d.clientsMu.clients[client]
	if !ok {
		return errors.New("the client is not registered")
	}

	shouldDistributeAllServices := len(clientRequestedServices) == 0

	var clusters []types.Resource
	shouldUpdateResourceVersion := len(services) == 0

	for _, service := range services {
		requestedListenerVersion, ok := clientRequestedServices[service.Name]
		if !ok && !shouldDistributeAllServices {
			continue
		}

		if !shouldDistributeAllServices && (requestedListenerVersion != service.Version) {
			shouldUpdateResourceVersion = true

			d.clientsMu.RUnlock()
			d.clientsMu.Lock()
			clientRequestedServices[service.Name] = service.Version
			d.clientsMu.Unlock()
			d.clientsMu.RLock()
		}

		if shouldDistributeAllServices {
			shouldUpdateResourceVersion = true
		}

		clusters = append(clusters, &cluster.Cluster{
			Name: service.DefaultRoute.Host,
			ClusterDiscoveryType: &cluster.Cluster_Type{
				Type: cluster.Cluster_LOGICAL_DNS,
			},
			LbPolicy: cluster.Cluster_ROUND_ROBIN,
			LoadAssignment: &endpoint.ClusterLoadAssignment{
				ClusterName: service.DefaultRoute.Host,
				Endpoints: []*endpoint.LocalityLbEndpoints{
					{
						LbEndpoints: []*endpoint.LbEndpoint{
							{
								HostIdentifier: &endpoint.LbEndpoint_Endpoint{
									Endpoint: &endpoint.Endpoint{
										Address: &core.Address{
											Address: &core.Address_SocketAddress{
												SocketAddress: &core.SocketAddress{
													Address: service.DefaultRoute.Host,
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
		})

		for _, route := range service.Routes {
			clusters = append(clusters, &cluster.Cluster{
				Name: route.Host,
				ClusterDiscoveryType: &cluster.Cluster_Type{
					Type: cluster.Cluster_LOGICAL_DNS,
				},
				LbPolicy: cluster.Cluster_ROUND_ROBIN,
				LoadAssignment: &endpoint.ClusterLoadAssignment{
					ClusterName: route.Host,
					Endpoints: []*endpoint.LocalityLbEndpoints{
						{
							LbEndpoints: []*endpoint.LbEndpoint{
								{
									HostIdentifier: &endpoint.LbEndpoint_Endpoint{
										Endpoint: &endpoint.Endpoint{
											Address: &core.Address{
												Address: &core.Address_SocketAddress{
													SocketAddress: &core.SocketAddress{
														Address: route.Host,
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
			})
		}
	}

	var version string
	if shouldUpdateResourceVersion {
		uid, err := uuid.NewRandom()
		if err != nil {
			return fmt.Errorf("failed to create a version for the snapshot cache: %w", err)
		}
		version = uid.String()
	} else {
		snapshot, err := d.snapshotCache.GetSnapshot(client)
		if err != nil {
			return fmt.Errorf("failed to get the cached snapshot for the client %s: %w", client, err)
		}
		version = snapshot.GetVersion(resource.ListenerType)
	}

	resources := map[resource.Type][]types.Resource{
		resource.ClusterType: clusters,
	}

	sc, err := cache.NewSnapshot(version, resources)
	if err != nil {
		return fmt.Errorf("failed to create a new snapshot: %w", err)
	}

	if err := d.snapshotCache.SetSnapshot(ctx, client, sc); err != nil {
		return fmt.Errorf("failed to create a snapshot cache to the client `%s`: %w", client, err)
	}

	return nil
}

func (d *ServiceDistributor) RegisterClient(ctx context.Context, client string, serviceNames []string) error {
	d.clientsMu.Lock()
	defer d.clientsMu.Unlock()

	_, ok := d.clientsMu.clients[client]
	if !ok || len(serviceNames) == 0 {
		d.clientsMu.clients[client] = make(map[string]string)
	}

	for _, service := range serviceNames {
		_, ok := d.clientsMu.clients[client][service]
		if !ok {
			d.clientsMu.clients[client][service] = ""
		}
	}

	return nil
}
