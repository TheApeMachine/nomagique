package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
)

func TestManifoldstate_Read(testingTB *testing.T) {
	Convey("Given dominant pressure gradient", testingTB, func() {
		stage := equation.NewManifoldstate(equation.ManifoldConfig())
		err := writeFeatureStage(stage, equation.ManifoldInputKeys, 12, 0.2, 0.5, 0.5, 50000)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should emit bounded category evidence", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 2)
			So(datura.Peek[float64](outbound, "output", "shockScore"), ShouldBeLessThan, 1)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeLessThan, 1)
		})
	})
}
