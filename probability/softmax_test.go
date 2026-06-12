package probability

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSoftmaxScores(testingTB *testing.T) {
	Convey("Given raw logits", testingTB, func() {
		probabilities, err := SoftmaxScores([]float64{1, 2, 3})

		sum := 0.0

		for _, probability := range probabilities {
			sum += probability
		}

		Convey("It should normalize to unity", func() {
			So(err, ShouldBeNil)
			So(sum, ShouldAlmostEqual, 1.0, 1e-9)
			So(ArgmaxIndex(probabilities), ShouldEqual, 2)
		})
	})

	Convey("Given non-finite logits", testingTB, func() {
		_, err := SoftmaxScores([]float64{1, math.NaN(), 3})

		So(err, ShouldNotBeNil)

		_, err = SoftmaxScores([]float64{1, math.Inf(1), 3})

		So(err, ShouldNotBeNil)
	})

	Convey("Given uniform logits", testingTB, func() {
		probabilities, err := SoftmaxScores([]float64{0, 0, 0, 0})

		Convey("CategoryConfidence should match the uniform share", func() {
			So(err, ShouldBeNil)
			So(len(probabilities), ShouldEqual, 4)

			confidence, err := CategoryConfidence(probabilities, 0)

			So(err, ShouldBeNil)
			So(confidence, ShouldAlmostEqual, 0.25, 1e-9)
		})
	})
}

func BenchmarkSoftmaxScores(testingTB *testing.B) {
	scores := []float64{0.6, 0.4, 0.7, 0.3}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = SoftmaxScores(scores)
	}
}
