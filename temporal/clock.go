package temporal

import (
	"sync"
	"time"
)

/*
Clock abstracts wall versus simulation time so runners and schedulers share
nanosecond Unix accessors without importing testing hooks everywhere.
*/
type Clock interface {
	NowUnixNano() int64
	NowUnix() int64
}

/*
Advanceable is implemented by clocks that support deterministic time advancement.
Runner uses this to drive iteration-based simulations without a type assertion.
*/
type Advanceable interface{ Advance(d time.Duration) }

/*
RealClock reads the process wall clock for live pipelines.
*/
type RealClock struct{}

/*
NowUnixNano exposes sub-second ordering keys aligned with data.Epoch sequencing.
*/
func (clock *RealClock) NowUnixNano() int64 {
	return time.Now().UnixNano()
}

/*
NowUnix exposes second-resolution timestamps for legacy consumers.
*/
func (clock *RealClock) NowUnix() int64 {
	return time.Now().Unix()
}

/*
SimulatedClock holds mutable simulation time and optionally drives an
EventScheduler when Advance moves the synthetic timeline forward.
*/
type SimulatedClock struct {
	current   int64
	mu        sync.RWMutex
	scheduler *EventScheduler
}

/*
NewSimulatedClock starts synthetic time at start nanoseconds since Unix epoch.
*/
func NewSimulatedClock(start int64) *SimulatedClock {
	return &SimulatedClock{
		current: start,
	}
}

/*
NowUnixNano returns the current simulation instant under the read lock.
*/
func (clock *SimulatedClock) NowUnixNano() int64 {
	clock.mu.RLock()
	defer clock.mu.RUnlock()
	return clock.current
}

/*
NowUnix truncates simulation nanoseconds to whole seconds.
*/
func (clock *SimulatedClock) NowUnix() int64 {
	clock.mu.RLock()
	defer clock.mu.RUnlock()
	return clock.current / 1e9
}

/*
Advance moves simulation time forward and drains scheduled callbacks up to the
new instant so causal handlers run before callers observe the updated clock.
*/
func (clock *SimulatedClock) Advance(delta time.Duration) {
	clock.mu.Lock()
	clock.current += int64(delta)
	newTime := clock.current
	clock.mu.Unlock()

	if clock.scheduler != nil {
		clock.scheduler.ProcessUntil(newTime)
	}
}

/*
SetTime jumps simulation time; callers must not regress below fired events unless
they have reset scheduler state consistently.
*/
func (clock *SimulatedClock) SetTime(t int64) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.current = t
}

/*
SetScheduler binds the heap scheduler so Advance automatically processes due
callbacks; must be wired before the first Advance when events are used.
*/
func (clock *SimulatedClock) SetScheduler(scheduler *EventScheduler) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.scheduler = scheduler
}
