package kernel

import (
	"math/rand"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveEMASamplesHotPhased_matchesInlined(testingTB *testing.T) {
	Convey("Given ready EMA and random samples", testingTB, func() {
		rng := rand.New(rand.NewSource(19))
		samples := make([]float64, 256)

		for index := range samples {
			samples[index] = rng.Float64()*200 - 100
		}

		Convey("When comparing phased and inlined drivers", func() {
			phasedState := EMAState{}
			inlineState := EMAState{}
			_ = ObserveEMA(&phasedState, 0)
			inlineState = phasedState

			phasedOut := make([]float64, len(samples))
			inlineOut := make([]float64, len(samples))

			observeEMASamplesHotPhased(&phasedState, samples, phasedOut)
			observeEMASamplesHotInlined(&inlineState, samples, inlineOut)

			Convey("It should match bit-for-bit", func() {
				phasedState.scratch = nil
				inlineState.scratch = nil
				So(phasedState, ShouldResemble, inlineState)

				for index := range samples {
					So(phasedOut[index], ShouldEqual, inlineOut[index])
				}
			})
		})
	})
}

func TestObserveDeltaSamplesHotPhased_matchesUnrolled(testingTB *testing.T) {
	Convey("Given ready delta and random samples", testingTB, func() {
		rng := rand.New(rand.NewSource(23))
		samples := make([]float64, 256)

		for index := range samples {
			samples[index] = rng.Float64()*200 - 100
		}

		Convey("When comparing phased and unrolled drivers", func() {
			phasedState := DeltaState{}
			unrolledState := DeltaState{}
			_ = ObserveDelta(&phasedState, 0)
			unrolledState = phasedState

			phasedOut := make([]float64, len(samples))
			unrolledOut := make([]float64, len(samples))

			observeDeltaSamplesHotPhased(&phasedState, samples, phasedOut)
			observeDeltaSamplesHotUnrolled(&unrolledState, samples, unrolledOut)

			Convey("It should match bit-for-bit", func() {
				phasedState.scratch = nil
				unrolledState.scratch = nil
				So(phasedState, ShouldResemble, unrolledState)

				for index := range samples {
					So(phasedOut[index], ShouldEqual, unrolledOut[index])
				}
			})
		})
	})
}

func TestObserveEMASamplesHot_usesPhasedAboveThreshold(testingTB *testing.T) {
	Convey("Given sample count at the vector threshold", testingTB, func() {
		So(vectorBatchThreshold, ShouldEqual, 32)
	})
}
