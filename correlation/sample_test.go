package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestSamplesFromNumbers(testingTB *testing.T) {
	Convey("Given paired timestamp/value numbers", testingTB, func() {
		samples, ok := samplesFromNumbers(core.Numbers{
			core.Float64(1.0),
			core.Float64(10.0),
			core.Float64(2.0),
			core.Float64(20.0),
		})

		Convey("It should decode async samples", func() {
			So(ok, ShouldBeTrue)
			So(len(samples), ShouldEqual, 2)
			So(samples[0].Value, ShouldEqual, 10)
			So(samples[1].Value, ShouldEqual, 20)
		})
	})
}
