package channel

import (
	"context"

	"github.com/kauche/cloud-run-service-router-xds/internal/domain/event"
)

var _ event.ServiceEventBroker = (*ServiceEventBroker)(nil)

type ServiceEventBroker struct{}

func NewServiceEventBroker() *ServiceEventBroker {
	return &ServiceEventBroker{}
}

func (s *ServiceEventBroker) PublishServicesRefreshedEvent(ctx context.Context) error {
	return nil
}

func (s *ServiceEventBroker) SubscribeServicesRefreshedEvent(ctx context.Context, handler func(ctx context.Context) error) error {
	return nil
}
