package nomagique

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/core"
)

func TestResolveStages_nestedScalar(testingTB *testing.T) {
	Convey("Given a nested Number wired to EMA", testingTB, func() {
		inner, err := Number(adaptive.EMA())

		So(err, ShouldBeNil)

		outer, err := Number(inner, adaptive.Delta())

		So(err, ShouldBeNil)

		Convey("It should flatten nested boundaries through the stage pool", func() {
			first := outer.Observe(inner, adaptive.Delta())
			second := Scalar(20).Observe(inner, adaptive.Delta())

			So(first, ShouldEqual, second)
		})
	})
}

func TestStageSlicePool_reuse(testingTB *testing.T) {
	Convey("Given repeated resolveStages calls", testingTB, func() {
		stages := []core.Number{adaptive.EMA(), adaptive.Delta()}

		first := resolveStages(stages)
		second := resolveStages(stages)

		Convey("It should return equivalent expanded stage lists", func() {
			So(first, ShouldResemble, second)
		})
	})
}
