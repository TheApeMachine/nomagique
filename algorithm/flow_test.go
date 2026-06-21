package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/tests"
)

func TestFlowEvaluate(testingTB *testing.T) {
	Convey("Given aggressive buy flow with rising price", testingTB, func() {
		flow := equation.NewFlow()
		writeErr := tests.WriteSamples(flow,
			500, 0, 5, 0, 100,
			100, 100.01, 100.02, 100.03, 100.04,
		)

		So(writeErr, ShouldBeNil)

		frame := make([]byte, 4096)
		_, _ = flow.Read(frame)
		outbound := datura.Acquire("test-out", datura.APPJSON)
		_, _ = outbound.Write(frame)

		Convey("It should favor aggressive drive", func() {
			drive := datura.Peek[float64](outbound, "output", "drive")
			absorption := datura.Peek[float64](outbound, "output", "absorption")

			So(drive, ShouldBeGreaterThan, 0)
			So(drive, ShouldBeGreaterThan, absorption)
		})
	})

	Convey("Given aggressive buy flow with flat price", testingTB, func() {
		flow := equation.NewFlow()
		writeErr := tests.WriteSamples(flow,
			200, 0, 4, 0, 50,
			50, 50.001, 50, 50.001,
		)

		So(writeErr, ShouldBeNil)

		frame := make([]byte, 4096)
		_, _ = flow.Read(frame)
		outbound := datura.Acquire("test-out", datura.APPJSON)
		_, _ = outbound.Write(frame)

		Convey("It should favor hidden absorption", func() {
			So(datura.Peek[float64](outbound, "output", "absorption"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkFlowRead(b *testing.B) {
	flow := equation.NewFlow()
	samples := []float64{
		500, 0, 5, 0, 100,
		100, 100.01, 100.02, 100.03, 100.04,
	}
	frame := make([]byte, 4096)

	b.ReportAllocs()

	for b.Loop() {
		_ = tests.WriteSamples(flow, samples...)
		_, _ = flow.Read(frame)
	}
}
