package event

import (
	"context"
)

type ServiceEventBroker interface {
	PublishServicesRefreshedEvent(ctx context.Context) error
	SubscribeServicesRefreshedEvent(ctx context.Context, subscriber func() error) error
}
