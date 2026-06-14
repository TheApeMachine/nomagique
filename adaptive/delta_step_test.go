package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveDelta(t *testing.T) {
	Convey("Given ObserveDelta", t, func() {
		state := DeltaState{}
		byFunc := ObserveDelta(&state, 10)
		byMethod := state.Observe(10)

		Convey("It should match method observation", func() {
			So(byFunc, ShouldEqual, byMethod)
		})
	})
}
