package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

func TestManifoldstate_Measure(testingTB *testing.T) {
	Convey("Given dominant pressure gradient", testingTB, func() {
		stage := equation.NewManifoldstate()
		frame := equation.NewFeatureFrame(
			equation.ManifoldInputKeys,
			[]float64{12, 0.2, 0.5, 0.5, 50000},
		)
		output, err := stage.Measure(frame)

		So(err, ShouldBeNil)

		Convey("It should emit bounded category evidence", func() {
			So(output.Category, ShouldEqual, 2)
			So(output.ShockScore, ShouldBeLessThan, 1)
			So(output.Strength, ShouldBeLessThan, 1)
		})
	})
}

func BenchmarkManifoldstateMeasure(testingTB *testing.B) {
	stage := equation.NewManifoldstate()
	frame := equation.NewFeatureFrame(
		equation.ManifoldInputKeys,
		[]float64{0.5, 8, 1, 0.5, 50000},
	)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = stage.Measure(frame)
	}
}
