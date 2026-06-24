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
		forecaster := Forecast(pairConfig("forecast-config"))

		Convey("It should return a usable dynamic", func() {
			So(forecaster, ShouldNotBeNil)
		})
	})
}

func TestForecasterRead(testingTB *testing.T) {
	Convey("Given empty inbound wire", testingTB, func() {
		forecaster := Forecast(pairConfig("forecast-config"))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, forecaster)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a fresh forecaster", testingTB, func() {
		forecaster := Forecast(pairConfig("forecast-config"))
		artifact := pairWire(datura.Acquire("test", datura.APPJSON), 10, 10)
		err := transport.NewFlipFlop(artifact, forecaster)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return unit scale", func() {
			So(got, ShouldEqual, 1)
			So(datura.Peek[float64](forecaster.artifact, "output", "scale"), ShouldEqual, 1)
		})
	})

	Convey("Given forecast history", testingTB, func() {
		forecaster := Forecast(pairConfig("forecast-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact = pairWire(artifact, 10, 10)
		_ = transport.NewFlipFlop(artifact, forecaster)

		artifact = pairWire(artifact, 10, 15)
		err := transport.NewFlipFlop(artifact, forecaster)

		So(err, ShouldBeNil)

		Convey("It should expose scale for feedback", func() {
			So(datura.Peek[float64](forecaster.artifact, "output", "scale"), ShouldBeGreaterThan, 1)
		})
	})

	Convey("Given zero actual with non-zero predicted", testingTB, func() {
		forecaster := Forecast(pairConfig("forecast-config"))
		artifact := pairWire(datura.Acquire("test", datura.APPJSON), 10, 0)
		err := transport.NewFlipFlop(artifact, forecaster)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given zero predicted", testingTB, func() {
		forecaster := Forecast(pairConfig("forecast-config"))
		artifact := pairWire(datura.Acquire("test", datura.APPJSON), 0, 10)
		err := transport.NewFlipFlop(artifact, forecaster)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestForecaster_learningComposition(testingTB *testing.T) {
	Convey("Given composed learning dynamics", testingTB, func() {
		trustWeight := Weight(pairConfig("trust-weight-config"))
		calibrator := SampleRatio(pairConfig("sample-ratio-config"))
		forecaster := Forecast(pairConfig("forecast-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact = pairWire(artifact, 10, 10)
		err := transport.NewFlipFlop(artifact, trustWeight)

		So(err, ShouldBeNil)

		artifact = pairWire(artifact, 10, 10)
		err = transport.NewFlipFlop(artifact, calibrator)

		So(err, ShouldBeNil)

		artifact = pairWire(artifact, 10, 15)
		err = transport.NewFlipFlop(artifact, calibrator)

		So(err, ShouldBeNil)

		artifact = pairWire(artifact, 10, 10)
		err = transport.NewFlipFlop(artifact, forecaster)

		So(err, ShouldBeNil)

		artifact = pairWire(artifact, 10, 15)
		err = transport.NewFlipFlop(artifact, forecaster)

		So(err, ShouldBeNil)

		Convey("It should raise learned scale", func() {
			So(datura.Peek[float64](forecaster.artifact, "output", "scale"), ShouldBeGreaterThan, 1)
		})
	})
}

func TestForecaster_withAdaptiveSignal(testingTB *testing.T) {
	Convey("Given EMA and forecast feedback", testingTB, func() {
		exponential := adaptive.NewEMA(datura.Acquire("ema-config", datura.APPJSON).Poke("sample", "input").Poke(2, "period").Poke(2, "smoothing"))
		forecaster := Forecast(pairConfig("forecast-config"))
		signal := scalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10)
		_ = transport.NewFlipFlop(signal, exponential)
		signal = scalarWire(datura.Acquire("test", datura.APPJSON), "sample", 12)
		err := transport.NewFlipFlop(signal, exponential)

		So(err, ShouldBeNil)

		level := datura.Peek[float64](signal, "output", "value")

		Convey("When comparing EMA level to the outcome", func() {
			outcome := pairWire(datura.Acquire("test", datura.APPJSON), level, 12)
			err := transport.NewFlipFlop(outcome, forecaster)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](forecaster.artifact, "output", "scale"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkForecastRead(testingTB *testing.B) {
	forecaster := Forecast(pairConfig("forecast-config-bench"))
	artifact := datura.Acquire("test", datura.APPJSON)

	artifact = pairWire(artifact, 10, 10)
	_ = transport.NewFlipFlop(artifact, forecaster)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact = pairWire(artifact, 10, 11)
		_ = transport.NewFlipFlop(artifact, forecaster)
	}
}
