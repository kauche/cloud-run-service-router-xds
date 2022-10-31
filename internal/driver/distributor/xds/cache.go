package xds

import cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"

func NewSnapshotCache() cache.SnapshotCache {
	// TODO: logger
	return cache.NewSnapshotCache(true, cache.IDHash{}, nil)
}
