package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestZScoreFromDeviation(testingTB *testing.T) {
	Convey("Given zero variance", testingTB, func() {
		Convey("It should return zero", func() {
			So(zScoreFromDeviation(5, 0), ShouldEqual, 0)
		})
	})
}

