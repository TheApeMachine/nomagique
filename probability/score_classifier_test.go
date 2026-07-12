package probability

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestScoreClassifier_Classify(t *testing.T) {
	Convey("Given a score classifier", t, func() {
		classifier := NewScoreClassifier(
			[]string{"alpha", "beta", "noise"},
			[]float64{10, 20, 30},
		)

		Convey("When scores are classified directly", func() {
			result, err := classifier.Classify(map[string]float64{
				"alpha":    0.2,
				"beta":     1.4,
				"noise":    0.1,
				"strength": 1.4,
			})

			Convey("It should return category, confidence, and distribution output", func() {
				So(err, ShouldBeNil)
				So(result.Category, ShouldEqual, 20)
				So(result.Confidence, ShouldBeGreaterThan, result.ConfidenceBaseline)
				So(result.Confidence, ShouldBeGreaterThan, result.EntryBaseline)
				So(result.EntryBaseline, ShouldBeGreaterThanOrEqualTo, result.ExitBaseline)
				So(result.Distribution, ShouldContainKey, "20")
			})
		})
	})

	Convey("Given a missing score", t, func() {
		classifier := NewScoreClassifier([]string{"alpha"}, nil)

		Convey("When classification is attempted", func() {
			_, err := classifier.Classify(map[string]float64{"strength": 1})

			Convey("It should return an error", func() {
				So(err, ShouldNotBeNil)
			})
		})
	})

	Convey("Given huge finite scores whose raw moments and evidence sum overflow", t, func() {
		classifier := NewScoreClassifier(
			[]string{"zero", "winner", "runner_up", "tail"},
			[]float64{10, 20, 30, 40},
		)

		Convey("When the winning score is not the first category", func() {
			result, err := classifier.Classify(map[string]float64{
				"zero":      0,
				"winner":    math.MaxFloat64 * 0.75,
				"runner_up": math.MaxFloat64 * 0.5,
				"tail":      math.MaxFloat64 * 0.25,
				"strength":  math.MaxFloat64 * 0.75,
			})

			Convey("It should return the finite winning category and adaptive gates", func() {
				So(err, ShouldBeNil)
				So(result.Category, ShouldEqual, 20)
				So(result.Confidence, ShouldAlmostEqual, 0.5, 1e-15)
				So(result.EntryBaseline, ShouldAlmostEqual, 1.0/3.0, 1e-15)
				So(result.ExitBaseline, ShouldAlmostEqual, 1.0/6.0, 1e-15)
				So(ArgmaxIndex(result.Probabilities), ShouldEqual, 1)
				So(math.IsNaN(result.Probabilities[1]), ShouldBeFalse)
				So(math.IsInf(result.Probabilities[1], 0), ShouldBeFalse)
			})
		})
	})
}

func BenchmarkScoreClassifier_Classify(b *testing.B) {
	classifier := NewScoreClassifier(
		[]string{"alpha", "beta", "noise"},
		[]float64{10, 20, 30},
	)
	scores := map[string]float64{
		"alpha":    0.2,
		"beta":     1.4,
		"noise":    0.1,
		"strength": 1.4,
	}

	b.ReportAllocs()

	for b.Loop() {
		if _, err := classifier.Classify(scores); err != nil {
			b.Fatal(err)
		}
	}
}
