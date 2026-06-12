package learn

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSampleRatioState_Observe(testingTB *testing.T) {
	Convey("Given a fresh sample-ratio state", testingTB, func() {
		state := SampleRatioState{}

		Convey("When bootstrapping a matched pair", func() {
			value := state.Observe(10, 10)

			Convey("It should return unit calibration", func() {
				So(value, ShouldEqual, 1)
			})
		})
	})

	Convey("Given sample-ratio history", testingTB, func() {
		state := SampleRatioState{}
		_ = state.Observe(10, 10)

		Convey("When the outcome wins", func() {
			value := state.Observe(10, 15)

			Convey("It should scale by actual over predicted within the ceiling", func() {
				So(value, ShouldBeGreaterThan, 1)
				So(value, ShouldBeLessThanOrEqualTo, 1.5)
			})
		})

		Convey("When the outcome loses", func() {
			lossState := SampleRatioState{}
			_ = lossState.Observe(10, 10)
			value := lossState.Observe(10, 5)

			Convey("It should preserve magnitude via one plus the ratio", func() {
				So(value, ShouldBeGreaterThan, 0)
				So(value, ShouldBeLessThanOrEqualTo, 1.5)
			})
		})
	})
}

func TestSampleRatioState_ObserveSamples(testingTB *testing.T) {
	Convey("Given pairs", testingTB, func() {
		state := SampleRatioState{}
		predicted := []float64{10, 10}
		actual := []float64{10, 15}
		out := make([]float64, len(predicted))

		Convey("When observing in batch", func() {
			state.ObserveSamples(predicted, actual, out)

			Convey("It should match sequential observation", func() {
				expect := SampleRatioState{}
				for index, predict := range predicted {
					So(out[index], ShouldEqual, expect.Observe(predict, actual[index]))
				}
			})
		})
	})
}

func TestObserveSampleRatio(testingTB *testing.T) {
	Convey("Given ObserveSampleRatio", testingTB, func() {
		byFunction := SampleRatioState{}
		byMethod := SampleRatioState{}

		Convey("It should match method observation", func() {
			So(
				ObserveSampleRatio(&byFunction, 10, 10),
				ShouldEqual,
				byMethod.Observe(10, 10),
			)
		})
	})
}

func BenchmarkSampleRatioState_Observe(testingTB *testing.B) {
	state := SampleRatioState{}
	_ = state.Observe(10, 10)

	for testingTB.Loop() {
		_ = state.Observe(10, 11)
	}
}

func BenchmarkSampleRatioState_ObserveSamples(testingTB *testing.B) {
	state := SampleRatioState{}
	predicted := make([]float64, 1024)
	actual := make([]float64, len(predicted))
	out := make([]float64, len(predicted))

	for index := range predicted {
		predicted[index] = 10
		actual[index] = 10 + float64(index%5)
	}

	for testingTB.Loop() {
		state.Reset()
		state.ObserveSamples(predicted, actual, out)
	}
}
