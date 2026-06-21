package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/tests"
)

func TestConvictionEvaluate(testingTB *testing.T) {
	Convey("Given broad positive breadth", testingTB, func() {
		conviction := equation.NewConviction()
		writeErr := tests.WriteSamples(conviction, 1.0, 2.0, 0.5, 1, 2.0)

		So(writeErr, ShouldBeNil)

		outbound := readOutbound(conviction)

		Convey("It should classify risk-on surge", func() {
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
		})
	})

	Convey("Given a local leader in a weak market", testingTB, func() {
		conviction := equation.NewConviction()
		writeErr := tests.WriteSamples(conviction, 0.33, 4.0, 0.5, 1, 4.0)

		So(writeErr, ShouldBeNil)

		outbound := readOutbound(conviction)

		Convey("It should classify divergent move", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 2)
		})
	})

	Convey("Given weak breadth without leadership", testingTB, func() {
		conviction := equation.NewConviction()
		writeErr := tests.WriteSamples(conviction, 0.2, -1.0, 0.5, 0, -1.0)

		So(writeErr, ShouldBeNil)

		outbound := readOutbound(conviction)

		Convey("It should classify systemic slump", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 3)
		})
	})
}

func BenchmarkConvictionRead(b *testing.B) {
	conviction := equation.NewConviction()
	samples := []float64{1.0, 2.0, 0.5, 1, 2.0}
	frame := make([]byte, 4096)

	b.ReportAllocs()

	for b.Loop() {
		_ = tests.WriteSamples(conviction, samples...)
		_, _ = conviction.Read(frame)
	}
}
