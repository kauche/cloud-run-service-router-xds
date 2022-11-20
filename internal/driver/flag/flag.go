package flag

import "time"

type Flags struct {
	Port       int
	Project    string
	Location   string
	SyncPeriod time.Duration
}
