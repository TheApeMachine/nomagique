package learning

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewRLS(testingTB *testing.T) {
	Convey("Given valid config", testingTB, func() {
		stage, err := NewRLS(RLSConfig{Dimension: 2, InitialVariance: 1000})

		Convey("It should return a usable learner", func() {
			So(err, ShouldBeNil)
			So(stage, ShouldNotBeNil)
		})
	})
}

func TestRLSMeasure(testingTB *testing.T) {
	Convey("Given invalid dimension", testingTB, func() {
		_, err := NewRLS(RLSConfig{Dimension: 0, InitialVariance: 1000})

		Convey("It should reject the config", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a one-dimensional linear stream", testingTB, func() {
		stage, err := NewRLS(RLSConfig{Dimension: 1, InitialVariance: 1000})
		So(err, ShouldBeNil)

		for _, feature := range []float64{1, 2, 3, 4, 5} {
			_, err = stage.Measure(RLSSample{
				Features: []float64{feature},
				Target:   2*feature + 1,
			})
			So(err, ShouldBeNil)
		}

		output, err := stage.Measure(RLSSample{
			Features: []float64{0.5},
			Target:   2,
		})

		Convey("It should retain coefficients and produce finite forecasts", func() {
			So(err, ShouldBeNil)
			So(math.IsNaN(output.Value), ShouldBeFalse)
			So(len(output.Beta), ShouldEqual, 2)
			So(len(output.CovarianceDiagonal), ShouldEqual, 2)
		})
	})
}

func TestRLSPredict(testingTB *testing.T) {
	Convey("Given a trained one-dimensional learner", testingTB, func() {
		stage, err := NewRLS(RLSConfig{Dimension: 1, InitialVariance: 1000})
		So(err, ShouldBeNil)

		for _, feature := range []float64{1, 2, 3, 4, 5} {
			_, err = stage.Measure(RLSSample{
				Features: []float64{feature},
				Target:   2*feature + 1,
			})
			So(err, ShouldBeNil)
		}

		before, err := stage.Predict([]float64{6})
		So(err, ShouldBeNil)
		after, err := stage.Predict([]float64{6})

		Convey("It should predict without changing retained state", func() {
			So(err, ShouldBeNil)
			So(after.Value, ShouldEqual, before.Value)
			So(after.Beta, ShouldResemble, before.Beta)
			So(after.Covariance, ShouldResemble, before.Covariance)
		})
	})
}

func BenchmarkRLSMeasure(b *testing.B) {
	stage, err := NewRLS(RLSConfig{
		Dimension:        3,
		InitialVariance:  1000,
		ForgettingFactor: 0.99,
	})

	if err != nil {
		b.Fatal(err)
	}

	sample := RLSSample{
		Features: []float64{1, 2, 3},
		Target:   4,
	}

	b.ReportAllocs()

	for b.Loop() {
		_, _ = stage.Measure(sample)
	}
}
