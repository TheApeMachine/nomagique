package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestForecast(testingTB *testing.T) {
	Convey("Given Forecast constructor", testingTB, func() {
		forecaster := Forecast()

		Convey("It should return a usable learner", func() {
			So(forecaster, ShouldNotBeNil)
		})
	})
}

func TestForecasterMeasure(testingTB *testing.T) {
	Convey("Given a fresh forecast learner", testingTB, func() {
		forecaster := Forecast()
		output, err := forecaster.Measure(LearningPair{Predicted: 10, Actual: 10})

		Convey("It should start from unit scale", func() {
			So(err, ShouldBeNil)
			So(output.Value, ShouldEqual, 1)
			So(output.Scale, ShouldEqual, 1)
		})
	})

	Convey("Given rising outcomes", testingTB, func() {
		forecaster := Forecast()
		_, err := forecaster.Measure(LearningPair{Predicted: 10, Actual: 10})
		So(err, ShouldBeNil)

		output, err := forecaster.Measure(LearningPair{Predicted: 10, Actual: 15})

		Convey("It should learn a larger scale", func() {
			So(err, ShouldBeNil)
			So(output.Scale, ShouldBeGreaterThan, 1)
		})
	})

	Convey("Given zero predicted", testingTB, func() {
		forecaster := Forecast()
		_, err := forecaster.Measure(LearningPair{Predicted: 0, Actual: 10})

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkForecastMeasure(testingTB *testing.B) {
	forecaster := Forecast()
	_, _ = forecaster.Measure(LearningPair{Predicted: 10, Actual: 10})

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = forecaster.Measure(LearningPair{Predicted: 10, Actual: 11})
	}
}
