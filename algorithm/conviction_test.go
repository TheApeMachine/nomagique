package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

func TestConvictionMeasure(testingTB *testing.T) {
	Convey("Given broad positive breadth with leadership", testingTB, func() {
		conviction := equation.NewConviction()
		output, err := conviction.Measure(equation.ConvictionInput{
			Breadth:        1.0,
			Change:         2.0,
			SurgeThreshold: 0.5,
			Leader:         true,
		})

		So(err, ShouldBeNil)

		Convey("It should classify risk-on surge", func() {
			So(output.Value, ShouldBeGreaterThan, 0)
			So(int(output.Category), ShouldEqual, 1)
		})
	})

	Convey("Given a local leader in a weak market", testingTB, func() {
		conviction := equation.NewConviction()
		output, err := conviction.Measure(equation.ConvictionInput{
			Breadth:        0.33,
			Change:         4.0,
			SurgeThreshold: 0.5,
			Leader:         true,
		})

		So(err, ShouldBeNil)

		Convey("It should classify divergent move", func() {
			So(int(output.Category), ShouldEqual, 2)
		})
	})

	Convey("Given weak breadth without leadership", testingTB, func() {
		conviction := equation.NewConviction()
		output, err := conviction.Measure(equation.ConvictionInput{
			Breadth:        0.2,
			Change:         -1.0,
			SurgeThreshold: 0.5,
			Leader:         false,
		})

		So(err, ShouldBeNil)

		Convey("It should classify systemic slump", func() {
			So(int(output.Category), ShouldEqual, 3)
		})
	})
}

func BenchmarkConvictionMeasure(benchmark *testing.B) {
	conviction := equation.NewConviction()
	input := equation.ConvictionInput{
		Breadth:        1.0,
		Change:         2.0,
		SurgeThreshold: 0.5,
		Leader:         true,
	}

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		_, _ = conviction.Measure(input)
	}
}
