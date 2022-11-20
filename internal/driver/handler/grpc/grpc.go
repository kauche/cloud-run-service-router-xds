package grpc

import (
	"context"
	"fmt"
	"net"

	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"

	"github.com/kauche/cloud-run-service-router-xds/internal/usecase"
)

func NewServer(ctx context.Context, uc *usecase.ServiceUseCase, sc cache.SnapshotCache, port int, logger logr.Logger) *Server {
	xdsServer := server.NewServer(ctx, sc, &callbacks{
		uc:            *uc,
		snapshotCache: sc,
		logger:        logger,
	})

	grpcServer := grpc.NewServer()

	discovery.RegisterAggregatedDiscoveryServiceServer(grpcServer, xdsServer)

	return &Server{
		port:       port,
		grpcServer: grpcServer,
	}
}

type Server struct {
	port       int
	grpcServer *grpc.Server
}

func (s *Server) Start(ctx context.Context) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen on the port: %w", err)
	}

	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("the server has aborted : %w", err)
	}

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.grpcServer.GracefulStop()

	return nil
}
