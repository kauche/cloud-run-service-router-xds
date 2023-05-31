package xds

import (
	"context"
	"errors"
	"fmt"
	"sync"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tls "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
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

		routes := make([]*route.Route, len(service.Routes)+1)
		clusters = make([]types.Resource, len(service.Routes)+1)

		utc := &tls.UpstreamTlsContext{
			CommonTlsContext: &tls.CommonTlsContext{
				ValidationContextType: &tls.CommonTlsContext_ValidationContext{
					ValidationContext: &tls.CertificateValidationContext{
						CaCertificateProviderInstance: &tls.CertificateProviderPluginInstance{
							InstanceName: "local", // TODO: This instance name should be configurable.
						},
					},
				},
			},
		}

		utcb, err := proto.Marshal(utc)
		if err != nil {
			return fmt.Errorf("failed to marshal UpstreamTlsContext: %w", err)
		}

		clusters[0] = &cluster.Cluster{
			Name: service.DefaultHost,
			ClusterDiscoveryType: &cluster.Cluster_Type{
				Type: cluster.Cluster_LOGICAL_DNS,
			},
			TransportSocket: &core.TransportSocket{
				Name: "envoy.transport_sockets.tls",
				ConfigType: &core.TransportSocket_TypedConfig{
					TypedConfig: &anypb.Any{
						TypeUrl: "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext",
						Value:   utcb,
					},
				},
			},
			LbPolicy: cluster.Cluster_ROUND_ROBIN,
			LoadAssignment: &endpoint.ClusterLoadAssignment{
				ClusterName: service.DefaultHost,
				Endpoints: []*endpoint.LocalityLbEndpoints{
					{
						LbEndpoints: []*endpoint.LbEndpoint{
							{
								HostIdentifier: &endpoint.LbEndpoint_Endpoint{
									Endpoint: &endpoint.Endpoint{
										Address: &core.Address{
											Address: &core.Address_SocketAddress{
												SocketAddress: &core.SocketAddress{
													Address: service.DefaultHost,
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

		i := 0
		for _, r := range service.Routes {
			routes[i] = &route.Route{
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
			}

			clusters[i+1] = &cluster.Cluster{
				Name: r.Host,
				ClusterDiscoveryType: &cluster.Cluster_Type{
					Type: cluster.Cluster_LOGICAL_DNS,
				},
				LbPolicy: cluster.Cluster_ROUND_ROBIN,
				LoadAssignment: &endpoint.ClusterLoadAssignment{
					ClusterName: r.Host,
					Endpoints: []*endpoint.LocalityLbEndpoints{
						{
							LbEndpoints: []*endpoint.LbEndpoint{
								{
									HostIdentifier: &endpoint.LbEndpoint_Endpoint{
										Endpoint: &endpoint.Endpoint{
											Address: &core.Address{
												Address: &core.Address_SocketAddress{
													SocketAddress: &core.SocketAddress{
														Address: r.Host,
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

			i++
		}

		routes[len(service.Routes)] = &route.Route{
			Match: &route.RouteMatch{
				PathSpecifier: &route.RouteMatch_Prefix{
					Prefix: "/",
				},
			},
			Action: &route.Route_Route{
				Route: &route.RouteAction{
					ClusterSpecifier: &route.RouteAction_Cluster{
						Cluster: service.DefaultHost,
					},
					Timeout: &duration.Duration{Seconds: 10}, // TODO: This timeout duration should be configurable.
				},
			},
		}

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
		resource.ClusterType:  clusters,
	}

	if len(listeners) != 0 {
		resources = map[resource.Type][]types.Resource{
			resource.ListenerType: listeners,
			resource.ClusterType:  clusters,
		}
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
