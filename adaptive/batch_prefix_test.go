package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestPrefixMinMax(testingTB *testing.T) {
	Convey("Given samples", testingTB, func() {
		samples := []float64{3, 1, 4, 2}
		minOut := make([]float64, len(samples))
		maxOut := make([]float64, len(samples))

		prefixMinMax(5, 0, samples, minOut, maxOut)

		Convey("It should track running extrema", func() {
			So(minOut, ShouldResemble, []float64{3, 1, 1, 1})
			So(maxOut, ShouldResemble, []float64{3, 3, 4, 4})
		})
	})
}
