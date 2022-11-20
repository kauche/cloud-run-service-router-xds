package xds

import (
	"fmt"

	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/log"
	"github.com/go-logr/logr"
)

func NewSnapshotCache(logger logr.Logger) cache.SnapshotCache {
	return cache.NewSnapshotCache(true, cache.IDHash{}, &snapshotCacheLogger{logger: logger})
}

var _ log.Logger = (*snapshotCacheLogger)(nil)

type snapshotCacheLogger struct {
	logger logr.Logger
}

func (s *snapshotCacheLogger) Debugf(format string, args ...interface{}) {
	s.logger.WithValues("level", "DEBUG").Info(fmt.Sprintf(format, args...))
}

func (s *snapshotCacheLogger) Infof(format string, args ...interface{}) {
	s.logger.WithValues("level", "INFO").Info(fmt.Sprintf(format, args...))
}

func (s *snapshotCacheLogger) Warnf(format string, args ...interface{}) {
	s.logger.WithValues("level", "WARN").Info(fmt.Sprintf(format, args...))
}

func (s *snapshotCacheLogger) Errorf(format string, args ...interface{}) {
	s.logger.WithValues("level", "ERROR").Info(fmt.Sprintf(format, args...))
}
