package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestAbsExact(testingTB *testing.T) {
	Convey("Given signed values", testingTB, func() {
		Convey("It should return exact magnitudes", func() {
			So(absExact(-2), ShouldEqual, 2)
			So(absExact(2), ShouldEqual, 2)
		})
	})
}
