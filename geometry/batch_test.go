package geometry

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObservePhaseVelocitySamples(testingTB *testing.T) {
	Convey("Given a phase-velocity state", testingTB, func() {
		state := PhaseVelocityState{}
		means := []float64{1, 2, 3}
		out := make([]float64, len(means))

		observePhaseVelocitySamples(&state, means, out)

		Convey("It should match sequential ObservePhaseVelocity", func() {
			expect := PhaseVelocityState{}

			for index, mean := range means {
				expectValue := ObservePhaseVelocity(&expect, mean)
				So(out[index], ShouldEqual, expectValue)
			}
		})
	})
}
