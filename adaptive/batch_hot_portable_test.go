package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveEMASamplesHotPortable(testingTB *testing.T) {
	Convey("Given a ready EMA state", testingTB, func() {
		state := EMAState{}
		_ = state.Observe(10)

		samples := []float64{11, 12, 13, 14, 15}
		out := make([]float64, len(samples))

		observeEMASamplesHot(&state, samples, out)

		Convey("It should match sequential observation", func() {
			expect := EMAState{}
			_ = expect.Observe(10)
			for index, sample := range samples {
				So(out[index], ShouldEqual, expect.Observe(sample))
			}
		})
	})
}
