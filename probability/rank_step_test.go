package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveRank(testingTB *testing.T) {
	Convey("Given a fresh rank state", testingTB, func() {
		state := RankState{}
		rank := ObserveRank(&state, 5)

		Convey("It should bootstrap with rank one", func() {
			So(state.Ready, ShouldBeTrue)
			So(rank, ShouldEqual, 1)
			So(state.Count, ShouldEqual, 1)
		})
	})

	Convey("Given ascending samples", testingTB, func() {
		state := RankState{}
		_ = ObserveRank(&state, 1)
		_ = ObserveRank(&state, 2)
		rank := ObserveRank(&state, 3)

		Convey("It should return maximum empirical rank", func() {
			So(rank, ShouldEqual, 1)
		})
	})

	Convey("Given a repeated minimum sample", testingTB, func() {
		state := RankState{}
		_ = ObserveRank(&state, 10)
		_ = ObserveRank(&state, 20)
		rank := ObserveRank(&state, 5)

		Convey("It should rank below all history", func() {
			So(rank, ShouldEqual, 1.0/3.0)
		})
	})

	Convey("Given a middle sample", testingTB, func() {
		state := RankState{}
		_ = ObserveRank(&state, 1)
		_ = ObserveRank(&state, 3)
		rank := ObserveRank(&state, 2)

		Convey("It should count at-or-below fraction", func() {
			So(rank, ShouldEqual, 2.0/3.0)
		})
	})
}
