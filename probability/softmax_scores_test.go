package probability

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSoftmaxScoresNormalized(testingTB *testing.T) {
	Convey("Given scores with different gaps", testingTB, func() {
		narrow, narrowErr := SoftmaxScoresNormalized([]float64{0, 1})
		wide, wideErr := SoftmaxScoresNormalized([]float64{0, 10})
		shifted, shiftedErr := SoftmaxScoresNormalized([]float64{1e10, 1e10 + 1})

		Convey("Max subtraction keeps the gap and a shifted pair matches", func() {
			So(narrowErr, ShouldBeNil)
			So(wideErr, ShouldBeNil)
			So(shiftedErr, ShouldBeNil)
			So(narrow[1], ShouldBeGreaterThan, narrow[0])
			So(wide[1], ShouldBeGreaterThan, narrow[1])
			So(shifted, ShouldResemble, narrow)
			So(narrow[0]+narrow[1], ShouldAlmostEqual, 1.0, 1e-15)
		})
	})

	Convey("Given huge finite scores", testingTB, func() {
		probabilities, err := SoftmaxScoresNormalized([]float64{
			0,
			math.MaxFloat64 * 0.75,
			math.MaxFloat64 * 0.5,
			math.MaxFloat64 * 0.25,
		})

		Convey("It stays finite and gives the max score the mass", func() {
			So(err, ShouldBeNil)
			So(ArgmaxIndex(probabilities), ShouldEqual, 1)
			So(probabilities[1], ShouldAlmostEqual, 1.0, 1e-12)
			So(probabilities[0]+probabilities[1]+probabilities[2]+probabilities[3],
				ShouldAlmostEqual, 1.0, 1e-12)
			So(math.IsNaN(probabilities[1]), ShouldBeFalse)
			So(math.IsInf(probabilities[1], 0), ShouldBeFalse)
		})
	})

	Convey("Given equal scores", testingTB, func() {
		probabilities, err := SoftmaxScoresNormalized([]float64{4, 4, 4, 4})

		Convey("It returns the uniform distribution", func() {
			So(err, ShouldBeNil)
			So(probabilities[0], ShouldAlmostEqual, 0.25, 1e-15)
			So(probabilities[3], ShouldAlmostEqual, 0.25, 1e-15)
		})
	})

	Convey("Given one score", testingTB, func() {
		probabilities, err := SoftmaxScoresNormalized([]float64{42})

		Convey("It returns certainty", func() {
			So(err, ShouldBeNil)
			So(probabilities, ShouldResemble, []float64{1})
		})
	})

	Convey("Given empty or non-finite scores", testingTB, func() {
		_, emptyErr := SoftmaxScoresNormalized(nil)
		_, nanErr := SoftmaxScoresNormalized([]float64{1, math.NaN()})

		Convey("It returns a validation error", func() {
			So(emptyErr, ShouldNotBeNil)
			So(nanErr, ShouldNotBeNil)
		})
	})
}

func BenchmarkSoftmaxScoresNormalized(testingTB *testing.B) {
	scores := []float64{
		0,
		math.MaxFloat64 * 0.75,
		math.MaxFloat64 * 0.5,
		math.MaxFloat64 * 0.25,
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if _, err := SoftmaxScoresNormalized(scores); err != nil {
			testingTB.Fatal(err)
		}
	}
}
