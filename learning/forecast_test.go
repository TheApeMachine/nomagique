package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/core"
)

func TestForecast(testingTB *testing.T) {
	Convey("Given Forecast constructor", testingTB, func() {
		forecaster := Forecast[float64]()

		Convey("It should return a usable dynamic", func() {
			So(forecaster, ShouldNotBeNil)
		})
	})
}

func TestForecaster_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		forecaster := Forecast[float64]()

		Convey("It should return zero output", func() {
			So(forecaster.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given a fresh forecaster", testingTB, func() {
		forecaster := Forecast[float64]()
		got := forecaster.Observe(numberInputs(10, 10)...)

		Convey("It should return unit scale", func() {
			So(float64(got), ShouldEqual, 1)
			So(forecaster.Scale(), ShouldEqual, 1)
		})
	})

	Convey("Given forecast history", testingTB, func() {
		forecaster := Forecast[float64]()
		_ = forecaster.Observe(numberInputs(10, 10)...)
		_ = forecaster.Observe(numberInputs(10, 15)...)

		Convey("It should expose scale for feedback", func() {
			So(forecaster.Scale(), ShouldBeGreaterThan, 1)
		})
	})
}

func TestForecaster_ObserveSamples(testingTB *testing.T) {
	Convey("Given a forecaster", testingTB, func() {
		forecaster := Forecast[float64]()
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
		forecaster := Forecast[float64]()
		_ = forecaster.Observe(numberInputs(10, 10)...)

		So(forecaster.Reset(), ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(forecaster.state.Ready, ShouldBeFalse)
			So(float64(forecaster.Observe()), ShouldEqual, 0)
		})
	})
}

func TestForecaster_learningComposition(testingTB *testing.T) {
	Convey("Given composed learning dynamics", testingTB, func() {
		trustWeight := Weight[float64]()
		calibrator := SampleRatio[float64]()
		forecaster := Forecast[float64]()

		_ = trustWeight.Observe(numberInputs(10, 10)...)
		_ = calibrator.Observe(numberInputs(10, 10)...)
		_ = calibrator.Observe(numberInputs(10, 15)...)
		_ = forecaster.Observe(numberInputs(10, 10)...)
		_ = forecaster.Observe(numberInputs(10, 15)...)

		Convey("It should raise learned scale", func() {
			So(forecaster.Scale(), ShouldBeGreaterThan, 1)
		})
	})
}

func TestForecaster_withAdaptiveSignal(testingTB *testing.T) {
	Convey("Given EMA and forecast feedback", testingTB, func() {
		exponential := adaptive.NewEMA[float64]()
		forecaster := Forecast[float64]()
		level := core.Scalar[float64](10).Observe(exponential)

		Convey("When comparing EMA level to the outcome", func() {
			_ = forecaster.Observe(level, core.Scalar[float64](12))

			So(forecaster.Scale(), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkForecast_Observe(testingTB *testing.B) {
	forecaster := Forecast[float64]()
	_ = forecaster.Observe(numberInputs(10, 10)...)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = forecaster.Observe(numberInputs(10, 11)...)
	}
}

func BenchmarkForecast_ObserveSamples(testingTB *testing.B) {
	forecaster := Forecast[float64]()
	predicted := make([]float64, 1024)
	actual := make([]float64, len(predicted))
	out := make([]float64, len(predicted))

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		forecaster.state.Reset()
		forecaster.ObserveSamples(predicted, actual, out)
	}
}
