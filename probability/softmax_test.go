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

func TestSoftmaxScoresNormalized(testingTB *testing.T) {
	Convey("Given a vector dominated by one unbounded score", testingTB, func() {
		raw := []float64{50, 0.5, 10, -5}

		plain, plainErr := SoftmaxScores(raw)
		So(plainErr, ShouldBeNil)
		plainConfidence, _ := CategoryConfidence(plain, 0)

		normalized, normErr := SoftmaxScoresNormalized(raw)
		So(normErr, ShouldBeNil)
		normConfidence, _ := CategoryConfidence(normalized, 0)

		Convey("Plain softmax saturates but normalized does not", func() {
			So(plainConfidence, ShouldAlmostEqual, 1.0, 1e-6)
			So(normConfidence, ShouldBeLessThan, plainConfidence)
			So(normConfidence, ShouldBeLessThan, 0.95)
		})

		Convey("The winning category is unchanged", func() {
			So(ArgmaxIndex(normalized), ShouldEqual, ArgmaxIndex(plain))
			So(ArgmaxIndex(normalized), ShouldEqual, 0)
		})

		Convey("It normalizes to unity", func() {
			sum := 0.0
			for _, p := range normalized {
				sum += p
			}
			So(sum, ShouldAlmostEqual, 1.0, 1e-9)
		})
	})

	Convey("Given a vector whose scale grows but whose shape is constant", testingTB, func() {
		small, _ := SoftmaxScoresNormalized([]float64{2, 1, 0.5, 0})
		large, _ := SoftmaxScoresNormalized([]float64{20, 10, 5, 0})

		Convey("Confidence is scale-invariant", func() {
			smallConfidence, _ := CategoryConfidence(small, 0)
			largeConfidence, _ := CategoryConfidence(large, 0)
			So(smallConfidence, ShouldAlmostEqual, largeConfidence, 1e-9)
		})
	})

	Convey("Given uniform scores", testingTB, func() {
		probabilities, err := SoftmaxScoresNormalized([]float64{3, 3, 3, 3})

		Convey("It falls back to the uniform distribution", func() {
			So(err, ShouldBeNil)
			confidence, _ := CategoryConfidence(probabilities, 0)
			So(confidence, ShouldAlmostEqual, 0.25, 1e-9)
		})
	})

	Convey("Given non-finite scores", testingTB, func() {
		_, err := SoftmaxScoresNormalized([]float64{1, math.Inf(1), 3})
		So(err, ShouldNotBeNil)
	})
}

func BenchmarkSoftmaxScores(testingTB *testing.B) {
	scores := []float64{0.6, 0.4, 0.7, 0.3}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = SoftmaxScores(scores)
	}
}
