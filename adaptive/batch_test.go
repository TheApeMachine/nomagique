package adaptive

import (
	"math/rand"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveSamples_bootstrapOnly(testingTB *testing.T) {
	Convey("Given a fresh state and one sample", testingTB, func() {
		state := EMAState{}
		samples := []float64{9}
		out := make([]float64, 1)

		observeSamples(&state, samples, out)

		Convey("It should bootstrap without entering the hot driver", func() {
			So(out[0], ShouldEqual, 9)
			So(state.Ready, ShouldBeTrue)
		})
	})
}

func TestObserveEMASamplesHot_matchesSequential(testingTB *testing.T) {
	Convey("Given ready EMA state and random samples", testingTB, func() {
		rng := rand.New(rand.NewSource(7))
		samples := make([]float64, 512)

		for index := range samples {
			samples[index] = rng.Float64()*100 - 50
		}

		Convey("When comparing asm batch to sequential ready steps", func() {
			batchState := EMAState{}
			_ = ObserveEMA(&batchState, 0)

			seqState := batchState
			batchOut := make([]float64, len(samples))
			seqOut := make([]float64, len(samples))

			observeEMASamplesHotPhased(&batchState, samples, batchOut)

			for index, sample := range samples {
				seqOut[index] = observeEMAReady(&seqState, sample)
			}

			Convey("It should match exactly", func() {
				batchState.scratch = nil
				seqState.scratch = nil
				So(batchState, ShouldResemble, seqState)

				for index := range samples {
					So(batchOut[index], ShouldEqual, seqOut[index])
				}
			})
		})
	})
}

func TestObserveEMASamplesHot_empty(testingTB *testing.T) {
	Convey("Given zero samples", testingTB, func() {
		state := EMAState{}
		_ = ObserveEMA(&state, 1)

		Convey("When observing hot batch", func() {
			observeEMASamplesHot(&state, nil, nil)

			Convey("It should return without panic", func() {
				So(state.Value, ShouldEqual, 1)
			})
		})
	})
}

func TestApplyEMAValuesFused_collapsedSpan(testingTB *testing.T) {
	Convey("Given zero span in prefix arrays", testingTB, func() {
		samples := []float64{4, 4}
		minOut := []float64{4, 4}
		maxOut := []float64{4, 4}
		out := make([]float64, 2)

		value, rate := applyEMAValuesFused(3, 3, samples, minOut, maxOut, out)

		Convey("It should keep value and zero rate", func() {
			So(out[0], ShouldEqual, 3)
			So(out[1], ShouldEqual, 3)
			So(value, ShouldEqual, 3)
			So(rate, ShouldEqual, 0)
		})
	})
}

func TestApplyEMAValuesFused_negativeDelta(testingTB *testing.T) {
	Convey("Given a decreasing sample", testingTB, func() {
		samples := []float64{10, 5}
		minOut := []float64{5, 5}
		maxOut := []float64{10, 10}
		out := make([]float64, 2)

		_, rate := applyEMAValuesFused(7, 10, samples, minOut, maxOut, out)

		Convey("It should use absolute delta", func() {
			So(rate, ShouldBeGreaterThan, 0)
			So(out[1], ShouldBeGreaterThan, 0)
		})
	})
}

func TestApplyDeltaOutputs_collapsedSpan(testingTB *testing.T) {
	Convey("Given zero span", testingTB, func() {
		samples := []float64{2, 2}
		minOut := []float64{2, 2}
		maxOut := []float64{2, 2}
		out := make([]float64, 2)

		applyDeltaOutputs(2, samples, minOut, maxOut, out)

		Convey("It should emit zero", func() {
			So(out[0], ShouldEqual, 0)
			So(out[1], ShouldEqual, 0)
		})
	})
}

func TestApplyDeltaOutputs_negativeDelta(testingTB *testing.T) {
	Convey("Given a decreasing sample", testingTB, func() {
		samples := []float64{10, 4}
		minOut := []float64{4, 4}
		maxOut := []float64{10, 10}
		out := make([]float64, 2)

		applyDeltaOutputs(10, samples, minOut, maxOut, out)

		Convey("It should normalize positive movement", func() {
			So(out[1], ShouldEqual, 1)
		})
	})
}

func TestObserveEMASamplesHotInlined_collapsedSpan(testingTB *testing.T) {
	Convey("Given ready EMA with zero span", testingTB, func() {
		state := EMAState{Value: 4, Prev: 4, Min: 4, Max: 4, Ready: true}
		out := make([]float64, 2)

		observeEMASamplesHotInlined(&state, []float64{4, 4}, out)

		Convey("It should keep value and advance prev", func() {
			So(out[0], ShouldEqual, 4)
			So(out[1], ShouldEqual, 4)
			So(state.Prev, ShouldEqual, 4)
		})
	})
}

func TestObserveDeltaSamples_bootstrapOnly(testingTB *testing.T) {
	Convey("Given a fresh delta and one sample", testingTB, func() {
		state := DeltaState{}
		samples := []float64{3}
		out := make([]float64, 1)

		observeDeltaSamples(&state, samples, out)

		Convey("It should bootstrap without entering the hot driver", func() {
			So(out[0], ShouldEqual, 0)
			So(state.Ready, ShouldBeTrue)
		})
	})
}

func TestObserveDeltaSamplesHot_matchesSequential(testingTB *testing.T) {
	Convey("Given ready delta state and random samples", testingTB, func() {
		rng := rand.New(rand.NewSource(11))
		samples := make([]float64, 512)

		for index := range samples {
			samples[index] = rng.Float64()*100 - 50
		}

		Convey("When comparing asm batch to sequential ready steps", func() {
			batchState := DeltaState{}
			_ = ObserveDelta(&batchState, 0)

			seqState := batchState
			batchOut := make([]float64, len(samples))
			seqOut := make([]float64, len(samples))

			observeDeltaSamplesHotPhased(&batchState, samples, batchOut)

			for index, sample := range samples {
				seqOut[index] = observeDeltaReady(&seqState, sample)
			}

			Convey("It should match exactly", func() {
				batchState.scratch = nil
				seqState.scratch = nil
				So(batchState, ShouldResemble, seqState)

				for index := range samples {
					So(batchOut[index], ShouldEqual, seqOut[index])
				}
			})
		})
	})
}
