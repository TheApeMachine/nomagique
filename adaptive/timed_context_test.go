package adaptive

import (
	"testing"
	"time"

	"github.com/theapemachine/nomagique/statistic"
	. "github.com/smartystreets/goconvey/convey"
)

func TestTimedContext_MatchWindow(testingTB *testing.T) {
	Convey("Given explicit trade span", testingTB, func() {
		timedContext := NewTimedContext()
		window := timedContext.MatchWindow(5 * time.Second)

		Convey("It should return the trade span", func() {
			So(window, ShouldEqual, 5*time.Second)
		})
	})

	Convey("Given trade interval ring history", testingTB, func() {
		timedContext := NewTimedContext()

		for _, interval := range []float64{1, 2, 3, 4} {
			timedContext.TradeIntervals.Observe(interval)
		}

		window := timedContext.MatchWindow(0)

		Convey("It should derive window from median and p75", func() {
			So(window, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given book pulse fallback", testingTB, func() {
		timedContext := NewTimedContext()

		for _, interval := range []float64{2, 3, 4} {
			timedContext.BookPulseIntervals.Observe(interval)
		}

		window := timedContext.MatchWindow(0)

		Convey("It should use book pulse median", func() {
			So(window, ShouldEqual, time.Duration(3*float64(time.Second)))
		})
	})

	Convey("Given empty rings", testingTB, func() {
		timedContext := NewTimedContext()

		Convey("It should return zero", func() {
			So(timedContext.MatchWindow(0), ShouldEqual, 0)
		})
	})
}

func TestTimedContext_MaxAge(testingTB *testing.T) {
	Convey("Given level lifetime ring history", testingTB, func() {
		timedContext := NewTimedContext()

		for _, lifetime := range []float64{10, 20, 30, 40} {
			timedContext.LevelLifetimes.Observe(lifetime)
		}

		maxAge := timedContext.MaxAge()

		Convey("It should return p75 level lifetime", func() {
			So(maxAge, ShouldBeGreaterThan, 0)
			So(maxAge, ShouldEqual, time.Duration(timedContext.LevelLifetimes.Quantile(0.75)*float64(time.Second)))
		})
	})

	Convey("Given book pulse fallback for max age", testingTB, func() {
		timedContext := NewTimedContext()

		for _, interval := range []float64{5, 6, 7} {
			timedContext.BookPulseIntervals.Observe(interval)
		}

		maxAge := timedContext.MaxAge()

		Convey("It should use book pulse median", func() {
			So(maxAge, ShouldEqual, time.Duration(6*float64(time.Second)))
		})
	})
}

func TestTimedContext_Cooldown(testingTB *testing.T) {
	Convey("Given match window and max age", testingTB, func() {
		timedContext := NewTimedContext()

		for _, lifetime := range []float64{4, 5, 6} {
			timedContext.LevelLifetimes.Observe(lifetime)
		}

		matchWindow := 2 * time.Second
		cooldown := timedContext.Cooldown(matchWindow)

		Convey("It should sum max age and match window", func() {
			So(cooldown, ShouldEqual, timedContext.MaxAge()+matchWindow)
		})
	})
}

func TestTimedContext_ObservationRingAdversarial(testingTB *testing.T) {
	Convey("Given non-positive ring observations", testingTB, func() {
		ring := statistic.NewObservationRing()
		ring.Observe(0)
		ring.Observe(-5)

		Convey("It should ignore invalid samples", func() {
			So(ring.Len(), ShouldEqual, 0)
		})
	})
}

func TestTimedContext_FlowSmoothingAlpha(testingTB *testing.T) {
	Convey("Given trade interval history", testingTB, func() {
		timedContext := NewTimedContext()

		for _, interval := range []float64{1, 2, 3} {
			timedContext.TradeIntervals.Observe(interval)
			timedContext.LevelLifetimes.Observe(interval * 2)
		}

		alpha := timedContext.FlowSmoothingAlpha(2*time.Second, 10*time.Second, 5)

		Convey("It should return a unit-interval smoothing alpha", func() {
			So(alpha, ShouldBeGreaterThan, 0)
			So(alpha, ShouldBeLessThan, 1)
		})
	})
}
