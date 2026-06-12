package decay

import (
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/timeline"
)

func TestIntensityAtAccumulatesDecayedImpulses(testingTB *testing.T) {
	Convey("Given a buy impulse before the horizon", testingTB, func() {
		start := time.Unix(0, 0)
		selfEvents := timeline.New([]time.Time{start})
		crossEvents := timeline.Timeline{}
		at := start.Add(2 * time.Second)

		Convey("It should raise intensity above the baseline", func() {
			intensity := IntensityAt(selfEvents, crossEvents, at, 1, 2, 0, 1)

			So(intensity, ShouldBeGreaterThan, 1)
		})
	})
}

func TestKernelSupportIncreasesWithHorizon(testingTB *testing.T) {
	Convey("Given one event and two horizons", testingTB, func() {
		start := time.Unix(0, 0)
		events := timeline.New([]time.Time{start})
		short := KernelSupport(events, start.Add(time.Second), 1)
		long := KernelSupport(events, start.Add(3*time.Second), 1)

		Convey("It should weight past events less as the horizon moves farther away", func() {
			So(long, ShouldBeLessThan, short)
		})
	})
}

func TestLogPositiveGuardsNonPositive(testingTB *testing.T) {
	Convey("Given a non-positive value", testingTB, func() {
		Convey("It should return a finite log", func() {
			So(math.IsInf(LogPositive(0), 0), ShouldBeFalse)
		})
	})
}
