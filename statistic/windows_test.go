package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRollingWindow_Resolve(testingTB *testing.T) {
	Convey("Given explicit window hints", testingTB, func() {
		rolling := NewRollingWindow(5, 60)
		shortWindow, longWindow, err := rolling.Resolve(nil)

		Convey("It should return the configured hints", func() {
			So(err, ShouldBeNil)
			So(shortWindow, ShouldEqual, 5)
			So(longWindow, ShouldEqual, 60)
		})
	})

	Convey("Given history without hints", testingTB, func() {
		history := []float64{1, 1, 1, 1, 10, 12, 9, 11}
		rolling := NewRollingWindow(0, 0)
		shortWindow, longWindow, err := rolling.Resolve(history)

		Convey("It should derive short and long windows from the sample spread", func() {
			So(err, ShouldBeNil)
			So(shortWindow, ShouldEqual, 3)
			So(longWindow, ShouldEqual, len(history))
		})
	})

	Convey("Given empty history without hints", testingTB, func() {
		rolling := NewRollingWindow(0, 0)
		_, _, err := rolling.Resolve(nil)

		Convey("It should reject missing history", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestRollingWindow_TargetLong(testingTB *testing.T) {
	Convey("Given a single sample without hints", testingTB, func() {
		rolling := NewRollingWindow(0, 0)
		required, err := rolling.TargetLong([]float64{10})

		Convey("It should require more than one sample before calibration", func() {
			So(err, ShouldBeNil)
			So(required, ShouldBeGreaterThan, 1)
		})
	})
}

func TestRollingWindow_ReturnLag(testingTB *testing.T) {
	Convey("Given history without an explicit return lag hint", testingTB, func() {
		rolling := NewRollingWindow(0, 0)
		lag, err := rolling.ReturnLag([]float64{1, 2, 3, 4, 5, 6, 7, 8}, 0)

		Convey("It should derive a positive lag from the long window", func() {
			So(err, ShouldBeNil)
			So(lag, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkRollingWindow_Resolve(testingTB *testing.B) {
	history := make([]float64, 128)
	rolling := NewRollingWindow(0, 0)

	for index := range history {
		history[index] = float64(index + 1)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _, _ = rolling.Resolve(history)
	}
}
