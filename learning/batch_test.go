package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveSampleRatioSamples(testingTB *testing.T) {
	Convey("Given a sample-ratio state", testingTB, func() {
		state := SampleRatioState{}
		predicted := []float64{10, 10}
		actual := []float64{10, 15}
		out := make([]float64, len(predicted))

		observeSampleRatioSamples(&state, predicted, actual, out)

		Convey("It should match sequential ObserveSampleRatio", func() {
			expect := SampleRatioState{}

			for index := range predicted {
				expectValue := ObserveSampleRatio(&expect, predicted[index], actual[index])
				So(out[index], ShouldEqual, expectValue)
			}
		})
	})
}

func TestObserveWeightSamples(testingTB *testing.T) {
	Convey("Given a weight state", testingTB, func() {
		state := WeightState{}
		predicted := []float64{10, 10}
		actual := []float64{10, 12}
		out := make([]float64, len(predicted))

		observeWeightSamples(&state, predicted, actual, out)

		Convey("It should match sequential ObserveWeight", func() {
			expect := WeightState{}

			for index := range predicted {
				expectValue := ObserveWeight(&expect, predicted[index], actual[index])
				So(out[index], ShouldEqual, expectValue)
			}
		})
	})
}

func TestObserveForecastSamples(testingTB *testing.T) {
	Convey("Given a forecast state", testingTB, func() {
		state := ForecastState{}
		predicted := []float64{10, 10}
		actual := []float64{10, 8}
		out := make([]float64, len(predicted))

		observeForecastSamples(&state, predicted, actual, out)

		Convey("It should match sequential ObserveForecast", func() {
			expect := ForecastState{}

			for index := range predicted {
				expectValue := ObserveForecast(&expect, predicted[index], actual[index])
				So(out[index], ShouldEqual, expectValue)
			}
		})
	})
}
