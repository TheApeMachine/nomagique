package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSamplesFromScalars(testingTB *testing.T) {
	Convey("Given paired timestamp/value scalars", testingTB, func() {
		samples, ok := samplesFromScalars([]float64{1.0, 10.0, 2.0, 20.0})

		Convey("It should decode async samples", func() {
			So(ok, ShouldBeTrue)
			So(len(samples), ShouldEqual, 2)
			So(samples[0].Value, ShouldEqual, 10)
			So(samples[1].Value, ShouldEqual, 20)
		})
	})

	Convey("Given too few values", testingTB, func() {
		_, ok := samplesFromScalars([]float64{1, 10})

		Convey("It should reject the input", func() {
			So(ok, ShouldBeFalse)
		})
	})
}
