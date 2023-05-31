package command

import (
	"context"
	"fmt"
	"os"

	"github.com/110y/run"
	"github.com/110y/servergroup"

	"github.com/kauche/cloud-run-service-router-xds/internal/driver/db/cloudrun"
	"github.com/kauche/cloud-run-service-router-xds/internal/driver/distributor/xds"
	"github.com/kauche/cloud-run-service-router-xds/internal/driver/env/envconfig"
	"github.com/kauche/cloud-run-service-router-xds/internal/driver/event/broker/gopubsub"
	"github.com/kauche/cloud-run-service-router-xds/internal/driver/event/subscriber"
	"github.com/kauche/cloud-run-service-router-xds/internal/driver/flag/flag"
	"github.com/kauche/cloud-run-service-router-xds/internal/driver/handler/grpc"
	"github.com/kauche/cloud-run-service-router-xds/internal/driver/log/zap"
	"github.com/kauche/cloud-run-service-router-xds/internal/driver/worker/ticker"
	"github.com/kauche/cloud-run-service-router-xds/internal/usecase"
)

const (
	exitCodeFailedToCreateLogger                    = 100
	exitCodeFailedToCreateCloudRunClient            = 101
	exitCodeFailedToSubscribeServicesRefreshedEvent = 102
	exitCodeFailedToGetFlags                        = 103
	exitCodeFailedToGetEnvironments                 = 104
	exitCodeServerAborted                           = 200
)

func Run() {
	run.Run(server)
}

func server(ctx context.Context) int {
	logger, err := zap.NewLogger()
	if err != nil {
		_, ferr := fmt.Fprintf(os.Stderr, "failed to create a logger: %s", err)
		if ferr != nil {
			// Unhandleable, something went wrong...
			panic(fmt.Sprintf("failed to write log:`%s` original error is:`%s`", ferr, err))
		}
		return exitCodeFailedToCreateLogger
	}

	commandLogger := logger.WithName("command")

	env, err := envconfig.GetEnvironments()
	if err != nil {
		commandLogger.Error(err, "failed to get environments")
		return exitCodeFailedToGetEnvironments
	}

	sc := xds.NewSnapshotCache(logger.WithName("snapshot_cache"))

	sd := xds.NewServiceDistributor(sc)

	sb := gopubsub.NewServiceEventBroker(logger.WithName("service_event_broker"))

	flags, err := flag.GetFlags()
	if err != nil {
		commandLogger.Error(err, "failed to get flags")
		return exitCodeFailedToGetFlags
	}

	sr, err := cloudrun.NewServiceRepository(ctx, flags.Project, flags.Location, env.CloudRunEmulatorHost)
	if err != nil {
		commandLogger.Error(err, "failed to create a cloud run client")
		return exitCodeFailedToCreateCloudRunClient
	}

	uc := usecase.NewServiceUseCase(sb, sd, sr)

	ss := subscriber.NewServiceEventSubscriber(uc, logger.WithName("service_event_subscriber"))

	st := ticker.NewServiceRefreshTicker(uc, flags.SyncPeriod, logger.WithName("service_refresh_ticker"))

	gs := grpc.NewServer(ctx, uc, sc, env.Port, logger.WithName("grpc_server"))

	if err := sb.SubscribeServicesRefreshedEvent(ctx, ss.ServicesRefreshedEventHandler); err != nil {
		commandLogger.Error(err, "failed to subscribe the service refreshed event")
		return exitCodeFailedToCreateCloudRunClient
	}

	var sg servergroup.Group

	sg.Add(st)
	sg.Add(gs)
	sg.Add(sb)

	if err := sg.Start(ctx); err != nil {
		commandLogger.Error(err, "the server has aborted")
		return exitCodeServerAborted
	}

	return 0
}
