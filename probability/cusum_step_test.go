package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveCUSUM(testingTB *testing.T) {
	Convey("Given a fresh CUSUM state", testingTB, func() {
		state := CUSUMState{}
		evidence := ObserveCUSUM(&state, 10)

		Convey("It should bootstrap without evidence", func() {
			So(state.Ready, ShouldBeTrue)
			So(state.Target, ShouldEqual, 10)
			So(evidence, ShouldEqual, 0)
		})
	})

	Convey("Given a downward move", testingTB, func() {
		state := CUSUMState{}
		_ = ObserveCUSUM(&state, 10)
		evidence := ObserveCUSUM(&state, 5)

		Convey("It should reset positive accumulation", func() {
			So(evidence, ShouldEqual, 0)
			So(state.Positive, ShouldEqual, 0)
		})
	})

	Convey("Given sustained upward drift", testingTB, func() {
		state := CUSUMState{}
		_ = ObserveCUSUM(&state, 10)
		_ = ObserveCUSUM(&state, 12)
		evidence := ObserveCUSUM(&state, 15)

		Convey("It should accumulate positive evidence", func() {
			So(evidence, ShouldBeGreaterThan, 0)
			So(state.Positive, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given zero span after bootstrap", testingTB, func() {
		state := CUSUMState{}
		_ = ObserveCUSUM(&state, 7)
		evidence := ObserveCUSUM(&state, 7)

		Convey("It should hold positive evidence without span", func() {
			So(evidence, ShouldEqual, 0)
			So(state.Positive, ShouldEqual, 0)
		})
	})
}
