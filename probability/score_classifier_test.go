package probability

import (
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
