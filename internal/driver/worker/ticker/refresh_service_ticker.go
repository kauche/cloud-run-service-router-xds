package ticker

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	"github.com/kauche/cloud-run-service-router-xds/internal/usecase"
)

type ServiceRefreshTicker struct {
	uc         *usecase.ServiceUseCase
	logger     logr.Logger
	syncPeriod time.Duration
}

func NewServiceRefreshTicker(uc *usecase.ServiceUseCase, syncPeriod time.Duration, logger logr.Logger) *ServiceRefreshTicker {
	return &ServiceRefreshTicker{
		uc:         uc,
		logger:     logger,
		syncPeriod: syncPeriod,
	}
}

func (t *ServiceRefreshTicker) tick(ctx context.Context) error {
	if err := t.uc.RefreshServices(ctx); err != nil {
		return fmt.Errorf("failed to refresh services: %s", err)
	}

	return nil
}

func (t *ServiceRefreshTicker) Start(ctx context.Context) error {
	ticker := time.NewTicker(t.syncPeriod)

	for {
		if err := t.tick(ctx); err != nil {
			t.logger.Error(err, "failed to tick")
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil
		}
	}
}
