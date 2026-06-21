package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/tests"
)

func TestBookflowEvaluate(testingTB *testing.T) {
	Convey("Given a bid-heavy book snapshot", testingTB, func() {
		bookflow := equation.NewBookflow()
		writeErr := tests.WriteSamples(bookflow,
			0.85, 0.80, 0.86, 1,
			100, 2, 12,
			0.8,
			4, 4, 4,
			0.80, 0.82, 0.84, 0.86,
			0.78, 0.79, 0.80, 0.81,
			0.80, 0.82, 0.83, 0.84,
		)

		So(writeErr, ShouldBeNil)

		outbound := readOutbound(bookflow)

		Convey("It should classify loaded imbalance", func() {
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
		})
	})

	Convey("Given deep bid wall with bearish touch", testingTB, func() {
		bookflow := equation.NewBookflow()
		writeErr := tests.WriteSamples(bookflow,
			0.6, -0.4, 0.5, 1,
			50, 2, 3,
			-0.5,
			4, 4, 4,
			0.6, 0.55, 0.58, 0.62,
			0.2, 0.18, 0.22, 0.19,
			0.25, 0.24, 0.26, 0.23,
		)

		So(writeErr, ShouldBeNil)

		outbound := readOutbound(bookflow)

		Convey("It should classify spoof trap", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 2)
		})
	})
}

func BenchmarkBookflowRead(b *testing.B) {
	bookflow := equation.NewBookflow()
	samples := []float64{
		0.85, 0.80, 0.86, 1,
		100, 2, 12,
		0.8,
		4, 4, 4,
		0.80, 0.82, 0.84, 0.86,
		0.78, 0.79, 0.80, 0.81,
		0.80, 0.82, 0.83, 0.84,
	}
	frame := make([]byte, 4096)

	b.ReportAllocs()

	for b.Loop() {
		_ = tests.WriteSamples(bookflow, samples...)
		_, _ = bookflow.Read(frame)
	}
}
