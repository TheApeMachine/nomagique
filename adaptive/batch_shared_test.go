package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveSamples(testingTB *testing.T) {
	Convey("Given EMA batch observation", testingTB, func() {
		state := EMAState{}
		samples := []float64{10, 5, 20}
		out := make([]float64, len(samples))

		observeSamples(&state, samples, out)

		Convey("It should match sequential observation", func() {
			expect := EMAState{}
			for index, sample := range samples {
				So(out[index], ShouldEqual, expect.Observe(sample))
			}
		})
	})
}
