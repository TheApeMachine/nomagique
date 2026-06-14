package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveEMA(t *testing.T) {
	Convey("Given ObserveEMA", t, func() {
		state := EMAState{}

		Convey("It should match method observation", func() {
			So(ObserveEMA(&state, 3), ShouldEqual, state.Observe(3))
		})
	})
}

