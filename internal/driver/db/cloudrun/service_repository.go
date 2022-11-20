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

	req := &runpb.ListServicesRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s", s.project, s.location),
	}
	iter := s.client.ListServices(ctx, req)

	servicesMap := make(map[string]*entity.Service)

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

		svc := &entity.Service{
			Name:        filepath.Base(service.Name),
			DefaultHost: uri.Host,
			Routes:      make(map[string]*entity.Route),
		}

		for _, status := range service.TrafficStatuses {
			if status.Tag == "" {
				continue
			}

			uri, err := url.Parse(status.Uri)
			if err != nil {
				return fmt.Errorf("failed to parse service uri for a tagged service: %w", err)
			}

			route := &entity.Route{
				Name: status.Tag,
				Host: uri.Host,
			}

			svc.Routes[route.Name] = route
		}

		oldSvc, ok := s.servicesMu.services[svc.Name]
		if ok && svc.Equal(oldSvc) {
			svc.Version = oldSvc.Version
		} else {
			version, err := uuid.NewRandom()
			if err != nil {
				return fmt.Errorf("failed to create a version for the snapshot cache: %w", err)
			}
			svc.Version = version.String()
		}

		servicesMap[svc.Name] = svc
	}

	s.servicesMu.services = servicesMap

	return nil
}
