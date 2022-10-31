package command

import (
	"context"

	"github.com/110y/run"
	"github.com/110y/servergroup"

	"github.com/kauche/cloud-run-service-router-xds/internal/driver/db/cloudrun"
	"github.com/kauche/cloud-run-service-router-xds/internal/driver/distributor/xds"
	"github.com/kauche/cloud-run-service-router-xds/internal/driver/event/broker/channel"
	"github.com/kauche/cloud-run-service-router-xds/internal/driver/event/subscriber"
	"github.com/kauche/cloud-run-service-router-xds/internal/driver/handler/grpc"
	"github.com/kauche/cloud-run-service-router-xds/internal/driver/worker/ticker"
	"github.com/kauche/cloud-run-service-router-xds/internal/usecase"
)

func Run() {
	run.Run(server)
}

func server(ctx context.Context) int {
	sc := xds.NewSnapshotCache()

	sd := xds.NewServiceDistributor(sc)

	sb := channel.NewServiceEventBroker()

	sr := cloudrun.NewServiceRepository()

	uc := usecase.NewServiceUseCase(sb, sd, sr)

	ss := subscriber.NewServiceEventSubscriber(uc)

	st := ticker.NewServiceRefreshTicker(uc)

	gs := grpc.NewServer(ctx, sc)

	if err := sb.SubscribeServicesRefreshedEvent(ctx, ss.ServicesRefreshedEventHandler); err != nil {
		// TODO:
	}

	var sg servergroup.Group
	sg.Add(st)
	sg.Add(gs)

	if err := sg.Start(ctx); err != nil {
		// TODO:
	}

	return 0
}
