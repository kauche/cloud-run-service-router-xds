package ticker

import (
	"context"

	"github.com/kauche/cloud-run-service-router-xds/internal/usecase"
)

type ServiceRefreshTicker struct {
	uc *usecase.ServiceUseCase
}

func NewServiceRefreshTicker(uc *usecase.ServiceUseCase) *ServiceRefreshTicker {
	return &ServiceRefreshTicker{
		uc: uc,
	}
}

func (t *ServiceRefreshTicker) tick(ctx context.Context) error {
	if err := t.uc.RefreshServices(ctx); err != nil {
		// TODO:
	}

	return nil
}

func (t *ServiceRefreshTicker) Start(ctx context.Context) error {
	return nil
}
