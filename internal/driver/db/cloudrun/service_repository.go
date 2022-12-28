package cloudrun

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"sync"

	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"google.golang.org/api/iterator"

	"github.com/kauche/cloud-run-service-router-xds/internal/domain/entity"
	"github.com/kauche/cloud-run-service-router-xds/internal/domain/repository"
)

const originServiceAnnotation = "kauche.com/cloud-run-service-router-origin-service"

var _ repository.ServiceRepository = (*ServiceRepository)(nil)

type ServiceRepository struct {
	client *run.ServicesClient

	project  string
	location string

	servicesMu struct {
		sync.RWMutex
		services map[string]*entity.Service
	}
}

func NewServiceRepository(ctx context.Context, project, location string) (*ServiceRepository, error) {
	client, err := run.NewServicesClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create a cloud run client: %w", err)
	}

	return &ServiceRepository{
		client:   client,
		project:  project,
		location: location,
	}, nil
}

func (s *ServiceRepository) ListAllServices(ctx context.Context) ([]*entity.Service, error) {
	s.servicesMu.RLock()
	defer s.servicesMu.RUnlock()

	return lo.Values(s.servicesMu.services), nil
}

func (s *ServiceRepository) RefreshServices(ctx context.Context) error {
	s.servicesMu.Lock()
	defer s.servicesMu.Unlock()

	// NOTE: since the paging is done by the client internally, we don't need to set PageSize and PageToken.
	req := &runpb.ListServicesRequest{
		Parent:      fmt.Sprintf("projects/%s/locations/%s", s.project, s.location),
		ShowDeleted: false,
	}
	iter := s.client.ListServices(ctx, req)

	servicesMap := make(map[string]*entity.Service)
	serviceNameToOriginServiceMap := make(map[string]*entity.Service)
	serviceNameToRouteServiceMap := make(map[string]map[string]*entity.Route)
	for {
		service, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to iterate services: %w", err)
		}

		uri, err := url.Parse(service.Uri)
		if err != nil {
			return fmt.Errorf("failed to parse service uri: %w", err)
		}

		serviceName := filepath.Base(service.Name)

		originServiceName, ok := service.Annotations[originServiceAnnotation]
		if ok {
			route := &entity.Route{
				Name: serviceName,
				Host: uri.Host,
			}
			_, ok := serviceNameToRouteServiceMap[originServiceName]
			if ok {
				serviceNameToRouteServiceMap[originServiceName][route.Name] = route
			} else {
				serviceNameToRouteServiceMap[originServiceName] = map[string]*entity.Route{
					route.Name: route,
				}
			}
		} else {
			serviceNameToOriginServiceMap[serviceName] = &entity.Service{
				Name:        serviceName,
				DefaultHost: uri.Host,
			}
		}

	}

	for name, originService := range serviceNameToOriginServiceMap {
		routes, ok := serviceNameToRouteServiceMap[name]
		if ok {
			originService.Routes = routes
		}

		oldSvc, ok := s.servicesMu.services[originService.Name]
		if ok && originService.Equal(oldSvc) {
			originService.Version = oldSvc.Version
		} else {
			version, err := uuid.NewRandom()
			if err != nil {
				return fmt.Errorf("failed to create a version for the snapshot cache: %w", err)
			}
			originService.Version = version.String()
		}

		servicesMap[originService.Name] = originService
	}

	s.servicesMu.services = servicesMap

	return nil
}
