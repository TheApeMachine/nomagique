package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCompetitionMargin(testingTB *testing.T) {
	Convey("Given positive excess and span", testingTB, func() {
		margin, err := CompetitionMargin(1, 1)

		Convey("It should return a value in (0, 1)", func() {
			So(err, ShouldBeNil)
			So(margin, ShouldEqual, 0.5)
		})
	})

	Convey("Given non-positive excess or span", testingTB, func() {
		margin, err := CompetitionMargin(0, 1)

		Convey("It should return an error", func() {
			So(err, ShouldNotBeNil)
			So(margin, ShouldEqual, 0)
		})
	})
}

func TestHypothesisSeparation(testingTB *testing.T) {
	Convey("Given equally strong competing hypotheses", testingTB, func() {
		ratio, err := HypothesisSeparation([]float64{0.8, 0.8, 0.1})

		Convey("It should report no separation", func() {
			So(err, ShouldBeNil)
			So(ratio, ShouldEqual, 0)
		})
	})

	Convey("Given one hypothesis clearly above its nearest competitor", testingTB, func() {
		ratio, err := HypothesisSeparation([]float64{0.8, 0.2, 0.1})

		Convey("It should report the dominant evidence not matched by a competitor", func() {
			So(err, ShouldBeNil)
			So(ratio, ShouldAlmostEqual, 0.75, 1e-12)
		})
	})

	Convey("Given an all-zero competition", testingTB, func() {
		ratio, err := HypothesisSeparation([]float64{0, 0})

		Convey("It should report no separation", func() {
			So(err, ShouldBeNil)
			So(ratio, ShouldEqual, 0)
		})
	})

	Convey("Given an invalid competition", testingTB, func() {
		_, missingErr := HypothesisSeparation([]float64{1})
		_, negativeErr := HypothesisSeparation([]float64{1, -1})

		Convey("It should reject missing or negative competing evidence", func() {
			So(missingErr, ShouldNotBeNil)
			So(negativeErr, ShouldNotBeNil)
		})
	})
}

func TestMagnitudeMargin(testingTB *testing.T) {
	Convey("Given a positive magnitude", testingTB, func() {
		margin, err := MagnitudeMargin(1)

		Convey("It should map into (0, 1)", func() {
			So(err, ShouldBeNil)
			So(margin, ShouldEqual, 0.5)
		})
	})

	Convey("Given a non-positive magnitude", testingTB, func() {
		margin, err := MagnitudeMargin(0)

		Convey("It should return an error", func() {
			So(err, ShouldNotBeNil)
			So(margin, ShouldEqual, 0)
		})
	})
}

func BenchmarkMagnitudeMargin(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_, _ = MagnitudeMargin(12.0)
	}
}

func BenchmarkHypothesisSeparation(benchmark *testing.B) {
	scores := []float64{0.8, 0.2, 0.1, 0.05}
	benchmark.ReportAllocs()

	for benchmark.Loop() {
		_, _ = HypothesisSeparation(scores)
	}
}
