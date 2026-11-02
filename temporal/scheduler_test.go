package temporal

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEventScheduler(t *testing.T) {
	Convey("Given a repeating event that cancels itself from its callback", t, func() {
		clock := NewSimulatedClock(0)
		scheduler := NewEventScheduler(clock)
		clock.SetScheduler(scheduler)

		fires := 0
		var id uint64
		id = scheduler.ScheduleRepeating(int64(time.Second), func() {
			fires++
			scheduler.Cancel(id)
		})

		clock.Advance(3 * time.Second)

		Convey("It does not reschedule the cancelled event", func() {
			So(fires, ShouldEqual, 1)
		})
	})

	Convey("Given a repeating event still waiting on the heap", t, func() {
		clock := NewSimulatedClock(0)
		scheduler := NewEventScheduler(clock)
		clock.SetScheduler(scheduler)

		fires := 0
		id := scheduler.ScheduleRepeating(int64(time.Second), func() { fires++ })
		scheduler.Cancel(id)
		clock.Advance(3 * time.Second)

		Convey("Cancel removes it before it fires", func() {
			So(fires, ShouldEqual, 0)
		})
	})

	Convey("Given a later event cancelled from an earlier callback", t, func() {
		clock := NewSimulatedClock(0)
		scheduler := NewEventScheduler(clock)
		clock.SetScheduler(scheduler)

		later := 0
		laterID := scheduler.Schedule(int64(2*time.Second), func() { later++ })
		scheduler.Schedule(int64(time.Second), func() { scheduler.Cancel(laterID) })
		clock.Advance(3 * time.Second)

		Convey("The later event stays cancelled", func() {
			So(later, ShouldEqual, 0)
		})
	})

	Convey("Given a positive repeating interval", t, func() {
		clock := NewSimulatedClock(0)
		scheduler := NewEventScheduler(clock)
		clock.SetScheduler(scheduler)

		fires := 0
		scheduler.ScheduleRepeating(int64(time.Second), func() { fires++ })
		clock.Advance(3 * time.Second)

		Convey("It fires once per interval through the advance", func() {
			So(fires, ShouldEqual, 3)
		})
	})

	Convey("Given a repeating event whose interval is not positive", t, func() {
		clock := NewSimulatedClock(0)
		scheduler := NewEventScheduler(clock)
		clock.SetScheduler(scheduler)

		zero := 0
		negative := 0
		scheduler.ScheduleRepeating(0, func() {
			zero++
			if zero > 3 {
				panic("interval 0 was rescheduled")
			}
		})
		scheduler.ScheduleRepeating(-5, func() {
			negative++
			if negative > 3 {
				panic("negative interval was rescheduled")
			}
		})

		clock.Advance(time.Second)

		Convey("It fires once and does not spin", func() {
			So(zero, ShouldEqual, 1)
			So(negative, ShouldEqual, 1)
		})
	})
}
