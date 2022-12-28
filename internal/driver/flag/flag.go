package flag

import "time"

type Flags struct {
	Project    string
	Location   string
	SyncPeriod time.Duration
}
