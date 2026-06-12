package prob

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRankState_Observe(testingTB *testing.T) {
	Convey("Given a fresh rank state", testingTB, func() {
		state := RankState{}

		Convey("When bootstrapping", func() {
			value := state.Observe(10)

			Convey("It should return unit rank", func() {
				So(value, ShouldEqual, 1)
			})
		})
	})

	Convey("Given rank history", testingTB, func() {
		state := RankState{}
		_ = state.Observe(10)
		value := state.Observe(5)

		Convey("It should return a lower rank probability", func() {
			So(value, ShouldBeLessThan, 1)
			So(value, ShouldBeGreaterThan, 0)
		})
	})
}

func TestRankState_ObserveSamples(testingTB *testing.T) {
	Convey("Given samples", testingTB, func() {
		state := RankState{}
		samples := []float64{10, 5, 15}
		out := make([]float64, len(samples))

		Convey("When observing in batch", func() {
			state.ObserveSamples(samples, out)

			Convey("It should match sequential observation", func() {
				expect := RankState{}
				for index, sample := range samples {
					So(out[index], ShouldEqual, expect.Observe(sample))
				}
			})
		})
	})
}

func BenchmarkRankState_Observe(testingTB *testing.B) {
	state := RankState{}
	_ = state.Observe(10)

	for testingTB.Loop() {
		_ = state.Observe(10.5)
	}
}
