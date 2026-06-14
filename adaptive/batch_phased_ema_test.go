package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveEMASamplesHotPhased(testingTB *testing.T) {
	Convey("Given a ready EMA state", testingTB, func() {
		state := EMAState{}
		_ = state.Observe(10)

		samples := []float64{11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27}
		out := make([]float64, len(samples))

		observeEMASamplesHotPhased(&state, samples, out)

		Convey("It should produce finite outputs", func() {
			for _, value := range out {
				So(value, ShouldBeGreaterThan, 0)
			}
		})
	})
}
