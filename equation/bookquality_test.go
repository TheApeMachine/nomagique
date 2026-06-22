package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
)

func TestBookQuality_Read(testingTB *testing.T) {
	Convey("Given near-touch toxic churn above gate", testingTB, func() {
		stage := equation.NewBookQuality(nil)
		writeErr := writeFeatureStage(stage, equation.BookQualityInputKeys,
			0, 0.1, 0, 0.1,
			80, 80,
			1, 4.5,
			0.15, 0.8, 0, 2,
			100,
		)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify toxic bluff", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 4.5)
		})
	})

	Convey("Given balanced depth with fills and no cancels", testingTB, func() {
		stage := equation.NewBookQuality(nil)
		writeErr := writeFeatureStage(stage, equation.BookQualityInputKeys,
			0, 0.1, 0, 0.1,
			80, 80,
			0, 0,
			0.15, 0, 0, 1,
			100,
		)

		So(writeErr, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify hard support", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 3)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 1)
		})
	})
}

func BenchmarkBookQualityRead(b *testing.B) {
	stage := equation.NewBookQuality(nil)
	values := []float64{
		0.3, 0.1, 0, 0,
		10, 10,
		0, 0,
		0.15, 0, 0, 2,
		50000,
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = writeFeatureStage(stage, equation.BookQualityInputKeys, values...)
		frame := make([]byte, 4096)
		_, _ = stage.Read(frame)
	}
}
