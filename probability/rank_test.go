package probability

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewRank(testingTB *testing.T) {
	Convey("Given Rank constructor", testingTB, func() {
		empirical := NewRank()

		Convey("It should return a usable rank model", func() {
			So(empirical, ShouldNotBeNil)
		})
	})
}

func TestRankMeasure(testingTB *testing.T) {
	Convey("Given empirical rank history", testingTB, func() {
		empirical := NewRank()
		_, err := empirical.Measure(10)

		So(err, ShouldBeNil)

		output, err := empirical.Measure(5)

		Convey("It should return the empirical probability at or below the sample", func() {
			So(err, ShouldBeNil)
			So(output.Value, ShouldEqual, 0.5)
			So(output.Count, ShouldEqual, 2)
		})
	})

	Convey("Given sub-unit empirical rank history", testingTB, func() {
		empirical := NewRank()
		var output RankOutput
		var err error

		for _, sample := range []float64{0.1, 0.2, 0.15} {
			output, err = empirical.Measure(sample)

			So(err, ShouldBeNil)
		}

		Convey("It should retain observations by count instead of value span", func() {
			So(output.Value, ShouldAlmostEqual, 2.0/3.0)
			So(output.Count, ShouldEqual, 3)
			So(len(empirical.history), ShouldEqual, 3)
		})
	})

	Convey("Given equal consecutive scalar samples", testingTB, func() {
		empirical := NewRank()

		for _, sample := range []float64{10, 10} {
			output, err := empirical.Measure(sample)

			So(err, ShouldBeNil)
			So(output.Ready, ShouldBeTrue)
		}

		output, err := empirical.Measure(10)

		Convey("It should still advance the observation count", func() {
			So(err, ShouldBeNil)
			So(output.Count, ShouldEqual, 3)
			So(output.Value, ShouldEqual, 1)
		})
	})

	Convey("Given non-finite input", testingTB, func() {
		empirical := NewRank()
		_, err := empirical.Measure(math.NaN())

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestRankReset(testingTB *testing.T) {
	Convey("Given reset after observation", testingTB, func() {
		empirical := NewRank()
		_, err := empirical.Measure(10)

		So(err, ShouldBeNil)

		empirical.Reset()

		Convey("It should clear derived state", func() {
			So(len(empirical.history), ShouldEqual, 0)
			So(empirical.minimum, ShouldEqual, 0)
			So(empirical.maximum, ShouldEqual, 0)
		})
	})
}

func BenchmarkRankMeasure(testingTB *testing.B) {
	empirical := NewRank()
	_, _ = empirical.Measure(10)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = empirical.Measure(10.5)
	}
}
