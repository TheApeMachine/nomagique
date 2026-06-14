package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestOLS_Fit(testingTB *testing.T) {
	Convey("Given a simple predictor", testingTB, func() {
		target := []float64{1, 2, 3}
		predictor := []float64{1, 2, 3}

		coefficients, ok := OLS(target, predictor)

		Convey("It should fit coefficients", func() {
			So(ok, ShouldBeTrue)
			So(len(coefficients), ShouldEqual, 2)
		})
	})
}
