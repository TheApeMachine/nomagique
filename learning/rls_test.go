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
			prior, measureErr := stage.Predict([]float64{feature})
			So(measureErr, ShouldBeNil)

			output, measureErr := stage.Measure(RLSSample{
				Features: []float64{feature},
				Target:   2*feature + 1,
			})
			So(measureErr, ShouldBeNil)
			So(output.Value, ShouldEqual, prior.Value)
		}

		before, err := stage.Predict([]float64{0.5})
		So(err, ShouldBeNil)
		output, err := stage.Measure(RLSSample{
			Features: []float64{0.5},
			Target:   2,
		})
		snapshot, snapErr := stage.Snapshot()

		Convey("It should forecast before observing and retain finite state", func() {
			So(err, ShouldBeNil)
			So(snapErr, ShouldBeNil)
			So(output.Value, ShouldEqual, before.Value)
			So(math.IsNaN(output.Value), ShouldBeFalse)
			So(len(snapshot.Beta), ShouldEqual, 2)
			So(len(snapshot.CovarianceDiagonal), ShouldEqual, 2)
		})
	})
}

func TestRLSPredict(testingTB *testing.T) {
	Convey("Given a trained one-dimensional learner", testingTB, func() {
		stage, err := NewRLS(RLSConfig{Dimension: 1, InitialVariance: 1000})
		So(err, ShouldBeNil)

		for _, feature := range []float64{1, 2, 3, 4, 5} {
			_, err = stage.Observe(RLSSample{
				Features: []float64{feature},
				Target:   2*feature + 1,
			})
			So(err, ShouldBeNil)
		}

		before, err := stage.Predict([]float64{6})
		So(err, ShouldBeNil)
		after, err := stage.Predict([]float64{6})
		beforeSnap, err := stage.Snapshot()
		So(err, ShouldBeNil)
		afterSnap, err := stage.Snapshot()

		Convey("It should predict without changing retained state", func() {
			So(err, ShouldBeNil)
			So(after.Value, ShouldEqual, before.Value)
			So(afterSnap.Beta, ShouldResemble, beforeSnap.Beta)
			So(afterSnap.Covariance, ShouldResemble, beforeSnap.Covariance)
		})
	})
}

func TestRLSObserveResetsTogether(testingTB *testing.T) {
	Convey("Given a learner forced through an unrecoverable update", testingTB, func() {
		stage, err := NewRLS(RLSConfig{Dimension: 1, InitialVariance: 1})
		So(err, ShouldBeNil)

		_, err = stage.Observe(RLSSample{
			Features: []float64{math.Inf(1)},
			Target:   1,
		})

		Convey("It should reject non-finite features without retaining half-reset state", func() {
			So(err, ShouldNotBeNil)
			snapshot, snapErr := stage.Snapshot()
			So(snapErr, ShouldBeNil)
			So(snapshot.Beta, ShouldResemble, []float64{0, 0})
			So(snapshot.CovarianceDiagonal[0], ShouldEqual, 1)
			So(snapshot.CovarianceDiagonal[1], ShouldEqual, 1)
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
