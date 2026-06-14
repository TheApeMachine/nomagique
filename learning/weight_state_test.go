package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestWeightState_Observe(testingTB *testing.T) {
	Convey("Given a fresh weight state", testingTB, func() {
		state := WeightState{}

		Convey("When bootstrapping", func() {
			value := state.Observe(10, 10)

			Convey("It should start fully trusting", func() {
				So(value, ShouldEqual, 1)
			})
		})
	})

	Convey("Given weight history", testingTB, func() {
		state := WeightState{}
		_ = state.Observe(10, 10)

		Convey("When predictions match", func() {
			value := state.Observe(10, 10)

			Convey("It should keep high trust", func() {
				So(value, ShouldBeGreaterThan, 0.9)
			})
		})

		Convey("When predictions diverge", func() {
			divergent := WeightState{}
			_ = divergent.Observe(10, 11)
			value := divergent.Observe(20, 30)

			Convey("It should lower trust", func() {
				So(value, ShouldBeLessThan, 0.9)
			})
		})
	})
}

func TestWeightState_ObserveSamples(testingTB *testing.T) {
	Convey("Given pairs", testingTB, func() {
		state := WeightState{}
		predicted := []float64{10, 10}
		actual := []float64{10, 20}
		out := make([]float64, len(predicted))

		Convey("When observing in batch", func() {
			state.ObserveSamples(predicted, actual, out)

			Convey("It should match sequential observation", func() {
				expect := WeightState{}
				for index, predict := range predicted {
					So(out[index], ShouldEqual, expect.Observe(predict, actual[index]))
				}
			})
		})
	})
}

func BenchmarkWeightState_Observe(testingTB *testing.B) {
	state := WeightState{}
	_ = state.Observe(10, 10)

	for testingTB.Loop() {
		_ = state.Observe(10, 12)
	}
}

func BenchmarkWeightState_ObserveSamples(testingTB *testing.B) {
	state := WeightState{}
	predicted := make([]float64, 1024)
	actual := make([]float64, len(predicted))
	out := make([]float64, len(predicted))

	for index := range predicted {
		predicted[index] = 10
		actual[index] = 10 + float64(index%7)
	}

	for testingTB.Loop() {
		state.Reset()
		state.ObserveSamples(predicted, actual, out)
	}
}
