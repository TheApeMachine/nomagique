package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
)

func TestBookflow_Read(testingTB *testing.T) {
	Convey("Given a bid-heavy book snapshot", testingTB, func() {
		stage := equation.NewBookflow(bookflowConfig())
		err := writeFeatureStage(stage, equation.BookflowInputKeys,
			0.85, 0.80, 0.86, 1,
			100, 2, 12,
			0.8,
			4, 4, 4,
			0.80, 0.82, 0.84, 0.86,
			0.78, 0.79, 0.80, 0.81,
			0.80, 0.82, 0.83, 0.84,
		)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify loaded imbalance", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given deep bid wall with bearish touch", testingTB, func() {
		stage := equation.NewBookflow(bookflowConfig())
		err := writeFeatureStage(stage, equation.BookflowInputKeys,
			0.6, -0.4, 0.5, 1,
			50, 2, 3,
			-0.5,
			4, 4, 4,
			0.6, 0.55, 0.58, 0.62,
			0.2, 0.18, 0.22, 0.19,
			0.25, 0.24, 0.26, 0.23,
		)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify spoof trap", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 2)
		})
	})

	Convey("Given weighted depth that collapses away from flat depth", testingTB, func() {
		stage := equation.NewBookflow(bookflowConfig())
		err := writeFeatureStage(stage, equation.BookflowInputKeys,
			0.8, 0.7, 0.1, 1,
			100, 2, 12,
			0.2,
			4, 4, 4,
			0.60, 0.62, 0.61, 0.63,
			0.60, 0.62, 0.61, 0.63,
			0.50, 0.48, 0.52, 0.50,
		)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify book thinning", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 3)
			So(datura.Peek[float64](outbound, "output", "thinScore"), ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given balanced depth below the loaded threshold", testingTB, func() {
		stage := equation.NewBookflow(bookflowConfig())
		err := writeFeatureStage(stage, equation.BookflowInputKeys,
			0.1, 0.05, 0.12, 1,
			100, 2, 12,
			0.0,
			4, 4, 4,
			0.50, 0.52, 0.51, 0.53,
			0.45, 0.46, 0.44, 0.47,
			0.50, 0.51, 0.49, 0.52,
		)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify dense neutrality", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 4)
			So(datura.Peek[float64](outbound, "output", "neutralScore"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkBookflowRead(b *testing.B) {
	stage := equation.NewBookflow(bookflowConfig())
	values := []float64{
		0.85, 0.80, 0.86, 1,
		100, 2, 12,
		0.8,
		4, 4, 4,
		0.80, 0.82, 0.84, 0.86,
		0.78, 0.79, 0.80, 0.81,
		0.80, 0.82, 0.83, 0.84,
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = writeFeatureStage(stage, equation.BookflowInputKeys, values...)
		frame := make([]byte, 4096)
		_, _ = stage.Read(frame)
	}
}
