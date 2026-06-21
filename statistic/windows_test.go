package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRollingWindows(t *testing.T) {
	Convey("Given explicit window hints", t, func() {
		shortWindow, longWindow := RollingWindows(nil, 5, 60)

		Convey("It should return the configured hints", func() {
			So(shortWindow, ShouldEqual, 5)
			So(longWindow, ShouldEqual, 60)
		})
	})

	Convey("Given history without hints", t, func() {
		history := []float64{1, 1, 1, 1, 10, 12, 9, 11}
		shortWindow, longWindow := RollingWindows(history, 0, 0)

		Convey("It should derive short and long windows from the sample spread", func() {
			So(shortWindow, ShouldEqual, 3)
			So(longWindow, ShouldEqual, len(history))
		})
	})

	Convey("Given empty history without hints", t, func() {
		shortWindow, longWindow := RollingWindows(nil, 0, 0)

		Convey("It should bootstrap minimal windows", func() {
			So(shortWindow, ShouldEqual, 2)
			So(longWindow, ShouldEqual, 3)
		})
	})

	Convey("Given a single sample without hints", t, func() {
		required := TargetLongWindow([]float64{10}, 0, 0)

		Convey("It should require more than one sample before calibration", func() {
			So(required, ShouldBeGreaterThan, 1)
		})
	})
}

func BenchmarkRollingWindows(b *testing.B) {
	history := make([]float64, 128)

	for index := range history {
		history[index] = float64(index + 1)
	}

	b.ReportAllocs()

	for b.Loop() {
		_, _ = RollingWindows(history, 0, 0)
	}
}
