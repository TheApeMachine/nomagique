package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCUSUMState_Observe(testingTB *testing.T) {
	Convey("Given a fresh CUSUM state", testingTB, func() {
		state := CUSUMState{}

		Convey("When bootstrapping", func() {
			value := state.Observe(10)

			Convey("It should return zero evidence", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})

	Convey("Given CUSUM history", testingTB, func() {
		state := CUSUMState{}
		_ = state.Observe(10)
		value := state.Observe(25)

		Convey("It should accumulate positive evidence", func() {
			So(value, ShouldBeGreaterThan, 0)
		})
	})
}

func TestCUSUMState_ObserveSamples(testingTB *testing.T) {
	Convey("Given samples", testingTB, func() {
		state := CUSUMState{}
		samples := []float64{10, 25}
		out := make([]float64, len(samples))

		Convey("When observing in batch", func() {
			state.ObserveSamples(samples, out)

			Convey("It should match sequential observation", func() {
				expect := CUSUMState{}
				for index, sample := range samples {
					So(out[index], ShouldEqual, expect.Observe(sample))
				}
			})
		})
	})
}

func BenchmarkCUSUMState_Observe(testingTB *testing.B) {
	state := CUSUMState{}
	_ = state.Observe(10)

	for testingTB.Loop() {
		_ = state.Observe(10.5)
	}
}
