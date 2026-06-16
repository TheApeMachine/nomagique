package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

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
