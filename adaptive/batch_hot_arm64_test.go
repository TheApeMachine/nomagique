//go:build arm64

package adaptive

import (
	"math/rand"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveEMASamplesHotARM64_empty(testingTB *testing.T) {
	Convey("Given zero samples", testingTB, func() {
		state := EMAState{}
		_ = ObserveEMA(&state, 2)

		Convey("When observing", func() {
			observeEMASamplesHot(&state, nil, nil)

			Convey("It should no-op", func() {
				So(state.Value, ShouldEqual, 2)
			})
		})
	})
}

func TestObserveEMASamplesHotARM64_matchesInlined(testingTB *testing.T) {
	Convey("Given ready EMA and random samples", testingTB, func() {
		rng := rand.New(rand.NewSource(31))
		samples := make([]float64, 1024)

		for index := range samples {
			samples[index] = rng.Float64()*200 - 100
		}

		Convey("When comparing arm64 hot asm to inlined Go", func() {
			asmState := EMAState{}
			inlineState := EMAState{}
			_ = ObserveEMA(&asmState, 0)
			inlineState = asmState

			asmOut := make([]float64, len(samples))
			inlineOut := make([]float64, len(samples))

			observeEMASamplesHot(&asmState, samples, asmOut)
			observeEMASamplesHotInlined(&inlineState, samples, inlineOut)

			Convey("It should match bit-for-bit", func() {
				asmState.scratch = nil
				inlineState.scratch = nil
				So(asmState, ShouldResemble, inlineState)

				for index := range samples {
					So(asmOut[index], ShouldEqual, inlineOut[index])
				}
			})
		})
	})
}
