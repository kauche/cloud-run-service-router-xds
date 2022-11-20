package gopubsub

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kauche/gopubsub"

	"github.com/kauche/cloud-run-service-router-xds/internal/domain/event"
)

var _ event.ServiceEventBroker = (*ServiceEventBroker)(nil)

type ServiceEventBroker struct {
	topic  *gopubsub.Topic[struct{}]
	logger logr.Logger
}

func NewServiceEventBroker(logger logr.Logger) *ServiceEventBroker {
	return &ServiceEventBroker{
		topic:  gopubsub.NewTopic[struct{}](),
		logger: logger,
	}
}

func (s *ServiceEventBroker) Start(ctx context.Context) error {
	s.topic.Start(ctx)

	return nil
}

func (s *ServiceEventBroker) PublishServicesRefreshedEvent(ctx context.Context) error {
	s.topic.Publish(struct{}{})

	return nil
}

func (s *ServiceEventBroker) SubscribeServicesRefreshedEvent(ctx context.Context, subscriber func() error) error {
	s.topic.Subscribe(func(struct{}) {
		if err := subscriber(); err != nil {
			s.logger.Error(err, "failed to handle event by the subscriber")
		}
	})

	return nil
}
