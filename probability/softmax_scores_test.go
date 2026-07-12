package probability

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSoftmaxScoresNormalized(testingTB *testing.T) {
	Convey("Given huge finite scores whose unscaled variance overflows", testingTB, func() {
		reference, referenceErr := SoftmaxScoresNormalized([]float64{0, 0.75, 0.5, 0.25})
		probabilities, err := SoftmaxScoresNormalized([]float64{
			0,
			math.MaxFloat64 * 0.75,
			math.MaxFloat64 * 0.5,
			math.MaxFloat64 * 0.25,
		})

		Convey("It should preserve the standardized distribution and winner", func() {
			So(referenceErr, ShouldBeNil)
			So(err, ShouldBeNil)
			So(ArgmaxIndex(probabilities), ShouldEqual, 1)
			So(probabilities, ShouldResemble, reference)
			So(probabilities[0]+probabilities[1]+probabilities[2]+probabilities[3],
				ShouldAlmostEqual, 1.0, 1e-15)
			So(math.IsNaN(probabilities[1]), ShouldBeFalse)
			So(math.IsInf(probabilities[1], 0), ShouldBeFalse)
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
