package hawkes

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/decay"
)

func TestExcitationState_DecayTo(testingTB *testing.T) {
	Convey("Given excitation accumulated at an earlier time", testingTB, func() {
		state := ExcitationState{
			buyToBuy:   2,
			sellToBuy:  1,
			buyToSell:  3,
			sellToSell: 4,
			lastTime:   time.Unix(0, 0),
			haveLast:   true,
		}
		later := state.lastTime.Add(time.Second)

		state.DecayTo(later, 1)

		Convey("It should scale all branches by exp(-beta * age)", func() {
			scale := decay.ExpNeg(1, 1)
			So(state.buyToBuy, ShouldAlmostEqual, 2*scale, 1e-12)
			So(state.sellToSell, ShouldAlmostEqual, 4*scale, 1e-12)
		})
	})
}

func TestExcitationState_LogLikelihoodSum(testingTB *testing.T) {
	Convey("Given a single buy event", testingTB, func() {
		start := time.Unix(0, 0)
		state := ExcitationState{}
		marked := []MarkedEvent{{At: start, Side: sideBuy}}

		logSum, ok := state.LogLikelihoodSum(
			marked,
			1, 1,
			0.1, 0.1, 0.1, 0.1,
			1,
		)

		Convey("It should accumulate log intensity at mu", func() {
			So(ok, ShouldBeTrue)
			So(logSum, ShouldEqual, 0)
		})
	})
}
