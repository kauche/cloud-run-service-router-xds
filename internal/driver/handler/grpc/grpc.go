package grpc

import (
	"context"

	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
)

func NewServer(ctx context.Context, sc cache.SnapshotCache) *Server {
	// TODO:
	server.NewServer(ctx, sc, nil)

	return &Server{}
}

type Server struct{}

func (s *Server) Start(ctx context.Context) error {
	return nil
}
