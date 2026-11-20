package data

import (
	"sync/atomic"
	"time"
)

var (
	Epoch  atomic.Uint64
	Tick   atomic.Uint64
	SeqIdx atomic.Uint64
)

/*
init seeds Epoch once at process start so every downstream ordering key shares
a common origin without racing lazy initialization on first tick.
*/
func init() {
	Epoch.Store(uint64(time.Now().UnixNano()))
}
