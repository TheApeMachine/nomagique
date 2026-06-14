package hawkes

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFitGatesFromHistory(testingTB *testing.T) {
	Convey("Given fit history", testingTB, func() {
		gates, ok := FitGatesFromHistory(
			[]float64{0.1, 0.2, 0.3, 0.4},
			[]float64{0.2, 0.3, 0.4, 0.5},
		)

		Convey("It should derive positive gates", func() {
			So(ok, ShouldBeTrue)
			So(gates.Ready(), ShouldBeTrue)
		})
	})
}
