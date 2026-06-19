package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/tests"
)

func TestDepthEvaluate(testingTB *testing.T) {
	Convey("Given deep quote volume versus peers", testingTB, func() {
		depth := equation.NewDepth()
		writeErr := tests.WriteSamples(depth,
			1200, 4,
			800, 900, 1000, 1100,
			1, 0,
		)

		So(writeErr, ShouldBeNil)

		frame := make([]byte, 4096)
		_, _ = depth.Read(frame)
		outbound := datura.Acquire("test-out", datura.APPJSON)
		_, _ = outbound.Write(frame)

		Convey("It should classify robust liquidity", func() {
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 3)
		})
	})

	Convey("Given peak scarcity volume", testingTB, func() {
		depth := equation.NewDepth()
		writeErr := tests.WriteSamples(depth,
			50, 3,
			1100, 950, 50,
			1, 0,
		)

		So(writeErr, ShouldBeNil)

		frame := make([]byte, 4096)
		_, _ = depth.Read(frame)
		outbound := datura.Acquire("test-out", datura.APPJSON)
		_, _ = outbound.Write(frame)

		Convey("It should classify extreme scarcity", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
		})
	})
}

func BenchmarkDepthRead(b *testing.B) {
	depth := equation.NewDepth()
	samples := []float64{
		1200, 4,
		800, 900, 1000, 1100,
		1, 0,
	}
	frame := make([]byte, 4096)

	b.ReportAllocs()

	for b.Loop() {
		_ = tests.WriteSamples(depth, samples...)
		_, _ = depth.Read(frame)
	}
}
