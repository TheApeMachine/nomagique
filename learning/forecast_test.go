package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/core"
)

func TestForecast(testingTB *testing.T) {
	Convey("Given Forecast constructor", testingTB, func() {
		forecaster := Forecast()

		Convey("It should return a usable dynamic", func() {
			So(forecaster, ShouldNotBeNil)
		})
	})
}

func TestForecaster_Observe(testingTB *testing.T) {
	Convey("Given a fresh forecaster", testingTB, func() {
		forecaster := Forecast()

		Convey("When bootstrapping", func() {
			value := forecaster.Observe(core.Float64(10), core.Float64(10))

			Convey("It should return unit scale", func() {
				So(value, ShouldEqual, 1)
				So(forecaster.Scale(), ShouldEqual, 1)
			})
		})
	})

	Convey("Given forecast history", testingTB, func() {
		forecaster := Forecast()
		forecaster.Observe(core.Float64(10), core.Float64(10))
		_ = forecaster.Observe(core.Float64(10), core.Float64(15))

		Convey("It should expose scale for feedback", func() {
			So(forecaster.Scale(), ShouldBeGreaterThan, 1)
		})
	})
}

func TestForecaster_ObserveSamples(testingTB *testing.T) {
	Convey("Given a forecaster", testingTB, func() {
		forecaster := Forecast()
		predicted := []float64{10, 10}
		actual := []float64{10, 15}
		out := make([]float64, len(predicted))

		Convey("When observing samples in batch", func() {
			forecaster.ObserveSamples(predicted, actual, out)

			Convey("It should fill the output buffer", func() {
				So(out[1], ShouldBeGreaterThan, 1)
			})
		})
	})
}

func TestForecaster_Reset(testingTB *testing.T) {
	Convey("Given a forecaster with state", testingTB, func() {
		forecaster := Forecast()
		forecaster.Observe(core.Float64(10), core.Float64(10))

		Convey("When reset", func() {
			err := forecaster.Reset()
So(err, ShouldBeNil)

			Convey("It should clear derived state", func() {
				So(forecaster.state.Ready, ShouldBeFalse)
			})
		})
	})
}

func TestForecaster_learningComposition(testingTB *testing.T) {
	Convey("Given composed learning dynamics", testingTB, func() {
		trustWeight := Weight()
		calibrator := SampleRatio()
		forecaster := Forecast()

		Convey("When observing settled outcomes", func() {
			trustWeight.Observe(core.Float64(10), core.Float64(10))
			_ = calibrator.Observe(core.Float64(10), core.Float64(10))
			_ = calibrator.Observe(core.Float64(10), core.Float64(15))
			_ = forecaster.Observe(core.Float64(10), core.Float64(10))
			_ = forecaster.Observe(core.Float64(10), core.Float64(15))

			Convey("It should raise learned scale", func() {
				So(forecaster.Scale(), ShouldBeGreaterThan, 1)
			})
		})
	})
}

func TestForecaster_withAdaptiveSignal(testingTB *testing.T) {
	Convey("Given EMA and forecast feedback", testingTB, func() {
		exponential := adaptive.EMA()
		forecaster := Forecast()
		number, err := nomagique.Number(exponential)
		So(err, ShouldBeNil)

		Convey("When comparing EMA level to the outcome", func() {
			number = nomagique.Scalar(10)
			level := number.Observe(exponential)
			_ = forecaster.Observe(level, core.Float64(12))

			Convey("It should evolve scale from explicit pairs", func() {
				So(forecaster.Scale(), ShouldBeGreaterThan, 0)
			})
		})
	})
}

func BenchmarkForecast_Observe(testingTB *testing.B) {
	forecaster := Forecast()
	forecaster.Observe(core.Float64(10), core.Float64(10))

	for testingTB.Loop() {
		forecaster.Observe(core.Float64(10), core.Float64(11))
	}
}

func BenchmarkForecast_ObserveSamples(testingTB *testing.B) {
	forecaster := Forecast()
	predicted := make([]float64, 1024)
	actual := make([]float64, len(predicted))
	out := make([]float64, len(predicted))

	for testingTB.Loop() {
		forecaster.state.Reset()
		forecaster.ObserveSamples(predicted, actual, out)
	}
}
