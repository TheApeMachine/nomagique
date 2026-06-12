package learning

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewRLSFilter(testingTB *testing.T) {
	Convey("Given a valid dimension", testingTB, func() {
		filter, err := NewRLSFilter(2, 1000)

		Convey("It should allocate the filter", func() {
			So(err, ShouldBeNil)
			So(filter, ShouldNotBeNil)
		})
	})

	Convey("Given a non-positive dimension", testingTB, func() {
		_, err := NewRLSFilter(0, 1000)

		Convey("It should return an error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestRLSFilterObserve(testingTB *testing.T) {
	Convey("Given a simple linear relation", testingTB, func() {
		filter, err := NewRLSFilter(1, 1000)
		So(err, ShouldBeNil)

		for step := 0; step < 32; step++ {
			feature := float64(step) / 32
			target := 2*feature + 1
			observeErr := filter.Observe([]float64{feature}, target)
			So(observeErr, ShouldBeNil)
		}

		forecast, predictErr := filter.Predict([]float64{0.5})

		Convey("It should learn the mapping", func() {
			So(predictErr, ShouldBeNil)
			So(forecast, ShouldAlmostEqual, 2, 0.25)
		})
	})

	Convey("Given a forgetting factor", testingTB, func() {
		filter, err := NewRLSFilter(1, 1000)
		So(err, ShouldBeNil)
		So(filter.SetForgettingFactor(0.5), ShouldBeNil)

		for step := 0; step < 16; step++ {
			observeErr := filter.Observe([]float64{1}, 1)
			So(observeErr, ShouldBeNil)
		}

		for step := 0; step < 16; step++ {
			observeErr := filter.Observe([]float64{1}, 5)
			So(observeErr, ShouldBeNil)
		}

		forecast, predictErr := filter.Predict([]float64{1})

		Convey("It should adapt faster to the new target", func() {
			So(predictErr, ShouldBeNil)
			So(forecast, ShouldBeGreaterThan, 2.5)
		})
	})

	Convey("Given repeated collinear updates with aggressive forgetting", testingTB, func() {
		filter, err := NewRLSFilter(13, 1000)
		So(err, ShouldBeNil)
		So(filter.SetForgettingFactor(0.01), ShouldBeNil)

		features := make([]float64, 13)

		for index := range features {
			features[index] = 0.42
		}

		for step := 0; step < 4096; step++ {
			target := 0.001 * float64(step%3-1)
			observeErr := filter.Observe(features, target)
			So(observeErr, ShouldBeNil)
		}

		forecast, predictErr := filter.Predict(features)

		Convey("It should stay numerically stable after repair", func() {
			So(predictErr, ShouldBeNil)
			So(math.IsNaN(forecast), ShouldBeFalse)
			So(math.IsInf(forecast, 0), ShouldBeFalse)
		})
	})
}

func BenchmarkRLSFilterObserve(b *testing.B) {
	filter, err := NewRLSFilter(13, 1000)

	if err != nil {
		b.Fatal(err)
	}

	if setErr := filter.SetForgettingFactor(0.01); setErr != nil {
		b.Fatal(setErr)
	}

	features := make([]float64, 13)

	for index := range features {
		features[index] = 0.42
	}

	b.ReportAllocs()

	for b.Loop() {
		target := 0.001 * float64(b.N%3-1)

		if observeErr := filter.Observe(features, target); observeErr != nil {
			b.Fatal(observeErr)
		}
	}
}
