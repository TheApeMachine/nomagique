package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestForecastState_Observe(testingTB *testing.T) {
	Convey("Given a fresh forecast state", testingTB, func() {
		state := ForecastState{}

		Convey("When bootstrapping", func() {
			value := state.Observe(10, 10)

			Convey("It should start at unit scale", func() {
				So(value, ShouldEqual, 1)
			})
		})
	})

	Convey("Given forecast history", testingTB, func() {
		state := ForecastState{}
		_ = state.Observe(10, 10)

		Convey("When actual exceeds predicted", func() {
			value := state.Observe(10, 15)

			Convey("It should raise the scale", func() {
				So(value, ShouldBeGreaterThan, 1)
			})
		})

		Convey("When actual falls short", func() {
			short := ForecastState{}
			_ = short.Observe(10, 10)
			value := short.Observe(10, 5)

			Convey("It should lower the scale", func() {
				So(value, ShouldBeLessThan, 1)
			})
		})
	})
}

func TestForecastState_ObserveSamples(testingTB *testing.T) {
	Convey("Given pairs", testingTB, func() {
		state := ForecastState{}
		predicted := []float64{10, 10}
		actual := []float64{10, 15}
		out := make([]float64, len(predicted))

		Convey("When observing in batch", func() {
			state.ObserveSamples(predicted, actual, out)

			Convey("It should match sequential observation", func() {
				expect := ForecastState{}
				for index, predict := range predicted {
					So(out[index], ShouldEqual, expect.Observe(predict, actual[index]))
				}
			})
		})
	})
}

func TestForecastState_Reset(testingTB *testing.T) {
	Convey("Given forecast state", testingTB, func() {
		state := ForecastState{}
		_ = state.Observe(10, 10)

		Convey("When reset", func() {
			state.Reset()

			Convey("It should clear readiness", func() {
				So(state.Ready, ShouldBeFalse)
			})
		})
	})
}

func TestObserveForecast(testingTB *testing.T) {
	Convey("Given ObserveForecast", testingTB, func() {
		byFunction := ForecastState{}
		byMethod := ForecastState{}

		Convey("It should match method observation", func() {
			So(
				ObserveForecast(&byFunction, 10, 10),
				ShouldEqual,
				byMethod.Observe(10, 10),
			)
		})
	})
}

func BenchmarkForecastState_Observe(testingTB *testing.B) {
	state := ForecastState{}
	_ = state.Observe(10, 10)

	for testingTB.Loop() {
		_ = state.Observe(10, 12)
	}
}

func BenchmarkForecastState_ObserveSamples(testingTB *testing.B) {
	state := ForecastState{}
	predicted := make([]float64, 1024)
	actual := make([]float64, len(predicted))
	out := make([]float64, len(predicted))

	for index := range predicted {
		predicted[index] = 10
		actual[index] = 10 + float64(index%7)
	}

	for testingTB.Loop() {
		state.Reset()
		state.ObserveSamples(predicted, actual, out)
	}
}
