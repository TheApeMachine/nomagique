package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestBindObserveSample(testingTB *testing.T) {
	Convey("Given EMA then delta stages", testingTB, func() {
		exponential := EMA()
		delta := Delta()
		apply := BindObserveSample([]core.Number{exponential, delta})
		So(apply, ShouldNotBeNil)

		Convey("It should match ObserveEMAThenDelta", func() {
			raw := 10.0
			So(apply(raw), ShouldEqual, ObserveEMAThenDelta(raw, exponential, delta))
		})
	})

	Convey("Given an unsupported stage count", testingTB, func() {
		apply := BindObserveSample([]core.Number{EMA(), Delta(), ZScore()})
		So(apply, ShouldBeNil)
	})
}
