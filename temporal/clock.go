package temporal

import (
	"sync"
	"time"
)

type Clock interface {
	NowUnixNano() int64
	NowUnix() int64
}

/*
Advanceable is implemented by clocks that support deterministic time advancement.
Runner uses this to drive iteration-based simulations without a type assertion.
*/
type Advanceable interface{ Advance(d time.Duration) }

type RealClock struct{}

func (clock *RealClock) NowUnixNano() int64 {
	return time.Now().UnixNano()
}

func (clock *RealClock) NowUnix() int64 {
	return time.Now().Unix()
}

type SimulatedClock struct {
	current   int64
	mu        sync.RWMutex
	scheduler *EventScheduler
}

func NewSimulatedClock(start int64) *SimulatedClock {
	return &SimulatedClock{
		current: start,
	}
}

func (clock *SimulatedClock) NowUnixNano() int64 {
	clock.mu.RLock()
	defer clock.mu.RUnlock()
	return clock.current
}

func (clock *SimulatedClock) NowUnix() int64 {
	clock.mu.RLock()
	defer clock.mu.RUnlock()
	return clock.current / 1e9
}

func (clock *SimulatedClock) Advance(delta time.Duration) {
	clock.mu.Lock()
	clock.current += int64(delta)
	newTime := clock.current
	clock.mu.Unlock()

	// Process all events up to new time
	if clock.scheduler != nil {
		clock.scheduler.ProcessUntil(newTime)
	}
}

func (clock *SimulatedClock) SetTime(t int64) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.current = t
}

// SetScheduler sets the event scheduler for this clock
// Must be called before Advance() if using event scheduling
func (clock *SimulatedClock) SetScheduler(scheduler *EventScheduler) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.scheduler = scheduler
}
