package learning

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/tests"
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
	Convey("Given empty Observe inputs", testingTB, func() {
		forecaster := Forecast()

		Convey("It should return zero output", func() {
			So(observeInputs(forecaster), ShouldEqual, 0)
		})
	})

	Convey("Given a fresh forecaster", testingTB, func() {
		forecaster := Forecast()
		got := observeInputs(forecaster, 10, 10)

		Convey("It should return unit scale", func() {
			So(float64(got), ShouldEqual, 1)
			So(forecaster.Scale(), ShouldEqual, 1)
		})
	})

	Convey("Given forecast history", testingTB, func() {
		forecaster := Forecast()
		_ = observeInputs(forecaster, 10, 10)
		_ = observeInputs(forecaster, 10, 15)

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
		_ = observeInputs(forecaster, 10, 10)

		So(forecaster.Reset(), ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(forecaster.state.Ready, ShouldBeFalse)
			So(float64(observeInputs(forecaster)), ShouldEqual, 0)
		})
	})
}

func TestForecaster_learningComposition(testingTB *testing.T) {
	Convey("Given composed learning dynamics", testingTB, func() {
		trustWeight := Weight()
		calibrator := SampleRatio()
		forecaster := Forecast()

		_ = observeInputs(trustWeight, 10, 10)
		_ = observeInputs(calibrator, 10, 10)
		_ = observeInputs(calibrator, 10, 15)
		_ = observeInputs(forecaster, 10, 10)
		_ = observeInputs(forecaster, 10, 15)

		Convey("It should raise learned scale", func() {
			So(forecaster.Scale(), ShouldBeGreaterThan, 1)
		})
	})
}

func TestForecaster_withAdaptiveSignal(testingTB *testing.T) {
	Convey("Given EMA and forecast feedback", testingTB, func() {
		exponential := adaptive.NewEMA()
		forecaster := Forecast()
		level, _ := tests.PipelineSample([]io.ReadWriter{exponential}, 10)

		Convey("When comparing EMA level to the outcome", func() {
			_ = observeWithWork(forecaster, level, 12)

			So(forecaster.Scale(), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkForecast_Observe(testingTB *testing.B) {
	forecaster := Forecast()
	_ = observeInputs(forecaster, 10, 10)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = observeInputs(forecaster, 10, 11)
	}
}

func BenchmarkForecast_ObserveSamples(testingTB *testing.B) {
	forecaster := Forecast()
	predicted := make([]float64, 1024)
	actual := make([]float64, len(predicted))
	out := make([]float64, len(predicted))

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		forecaster.state.Reset()
		forecaster.ObserveSamples(predicted, actual, out)
	}
}
