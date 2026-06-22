package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/adaptive"
)

func TestForecast(testingTB *testing.T) {
	Convey("Given Forecast constructor", testingTB, func() {
		forecaster := Forecast(datura.Acquire("forecast-config", datura.APPJSON))

		Convey("It should return a usable dynamic", func() {
			So(forecaster, ShouldNotBeNil)
		})
	})
}

func TestForecaster_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		forecaster := Forecast(datura.Acquire("forecast-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, forecaster)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a fresh forecaster", testingTB, func() {
		forecaster := Forecast(datura.Acquire("forecast-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(10, "sample").
			Poke(10, "paired")
		err := transport.NewFlipFlop(artifact, forecaster)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return unit scale", func() {
			So(got, ShouldEqual, 1)
			So(forecaster.Scale(), ShouldEqual, 1)
		})
	})

	Convey("Given forecast history", testingTB, func() {
		forecaster := Forecast(datura.Acquire("forecast-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(10, "sample").Poke(10, "paired")
		err := transport.NewFlipFlop(artifact, forecaster)

		So(err, ShouldBeNil)

		artifact.Poke(10, "sample").Poke(15, "paired")
		err = transport.NewFlipFlop(artifact, forecaster)

		So(err, ShouldBeNil)

		Convey("It should expose scale for feedback", func() {
			So(forecaster.Scale(), ShouldBeGreaterThan, 1)
		})
	})
}

func TestForecaster_ObserveSamples(testingTB *testing.T) {
	Convey("Given a forecaster", testingTB, func() {
		forecaster := Forecast(datura.Acquire("forecast-config", datura.APPJSON))
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
		forecaster := Forecast(datura.Acquire("forecast-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(10, "sample").
			Poke(10, "paired")
		err := transport.NewFlipFlop(artifact, forecaster)

		So(err, ShouldBeNil)
		So(forecaster.Reset(), ShouldBeNil)

		fresh := datura.Acquire("test", datura.APPJSON).
			Poke(10, "sample").
			Poke(10, "paired")
		err = transport.NewFlipFlop(fresh, forecaster)

		So(err, ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(forecaster.Scale(), ShouldEqual, 1)
			So(datura.Peek[float64](fresh, "output", "value"), ShouldEqual, 1)
		})
	})
}

func TestForecaster_learningComposition(testingTB *testing.T) {
	Convey("Given composed learning dynamics", testingTB, func() {
		trustWeight := Weight(datura.Acquire("trust-weight-config", datura.APPJSON))
		calibrator := SampleRatio(datura.Acquire("sample-ratio-config", datura.APPJSON))
		forecaster := Forecast(datura.Acquire("forecast-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(10, "sample").Poke(10, "paired")
		err := transport.NewFlipFlop(artifact, trustWeight)

		So(err, ShouldBeNil)

		artifact.Poke(10, "sample").Poke(10, "paired")
		err = transport.NewFlipFlop(artifact, calibrator)

		So(err, ShouldBeNil)

		artifact.Poke(10, "sample").Poke(15, "paired")
		err = transport.NewFlipFlop(artifact, calibrator)

		So(err, ShouldBeNil)

		artifact.Poke(10, "sample").Poke(10, "paired")
		err = transport.NewFlipFlop(artifact, forecaster)

		So(err, ShouldBeNil)

		artifact.Poke(10, "sample").Poke(15, "paired")
		err = transport.NewFlipFlop(artifact, forecaster)

		So(err, ShouldBeNil)

		Convey("It should raise learned scale", func() {
			So(forecaster.Scale(), ShouldBeGreaterThan, 1)
		})
	})

	Convey("Given zero actual with non-zero predicted", testingTB, func() {
		forecaster := Forecast(datura.Acquire("forecast-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(10, "sample").
			Poke(0, "paired")
		err := transport.NewFlipFlop(artifact, forecaster)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given zero predicted", testingTB, func() {
		forecaster := Forecast(datura.Acquire("forecast-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(0, "sample").
			Poke(10, "paired")
		err := transport.NewFlipFlop(artifact, forecaster)

		Convey("It should return a parse error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestForecaster_withAdaptiveSignal(testingTB *testing.T) {
	Convey("Given EMA and forecast feedback", testingTB, func() {
		exponential := adaptive.NewEMA(datura.Acquire("ema-config", datura.APPJSON))
		forecaster := Forecast(datura.Acquire("forecast-config", datura.APPJSON))
		signal := datura.Acquire("test", datura.APPJSON).Poke(10, "sample")
		err := transport.NewFlipFlop(signal, exponential)

		So(err, ShouldBeNil)

		level := datura.Peek[float64](signal, "output", "value")

		Convey("When comparing EMA level to the outcome", func() {
			outcome := datura.Acquire("test", datura.APPJSON).
				Poke(level, "sample").
				Poke(12, "paired")
			err := transport.NewFlipFlop(outcome, forecaster)

			So(err, ShouldBeNil)
			So(forecaster.Scale(), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkForecast_Observe(testingTB *testing.B) {
	forecaster := Forecast(datura.Acquire("forecast-config-bench", datura.APPJSON))
	artifact := datura.Acquire("test", datura.APPJSON)

	artifact.Poke(10, "sample").Poke(10, "paired")
	_ = transport.NewFlipFlop(artifact, forecaster)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact.Poke(10, "sample").Poke(11, "paired")
		_ = transport.NewFlipFlop(artifact, forecaster)
	}
}

func BenchmarkForecast_ObserveSamples(testingTB *testing.B) {
	forecaster := Forecast(datura.Acquire("forecast-config-bench", datura.APPJSON))
	predicted := make([]float64, 1024)
	actual := make([]float64, len(predicted))
	out := make([]float64, len(predicted))

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		forecaster.state.Reset()
		forecaster.ObserveSamples(predicted, actual, out)
	}
}
