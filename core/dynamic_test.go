package core

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNumbersFloat64(testingTB *testing.T) {
	Convey("Given boundary numbers", testingTB, func() {
		values := Numbers{Float64(1), Float64(2)}.Float64()

		Convey("It should read raw samples", func() {
			So(values, ShouldResemble, []float64{1, 2})
		})
	})
}
