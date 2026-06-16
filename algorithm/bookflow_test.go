package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/tests"
)

func TestBookflowEvaluate(testingTB *testing.T) {
	Convey("Given a bid-heavy book snapshot", testingTB, func() {
		bookflow := NewBookflow()
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
		_, _ = bookflow.Read(make([]byte, 4096))

		Convey("It should classify loaded imbalance", func() {
			So(bookflow.outcome.Eligible, ShouldBeTrue)
			So(bookflow.outcome.Category, ShouldEqual, 1)
			So(bookflow.outcome.Strength, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given deep bid wall with bearish touch", testingTB, func() {
		bookflow := NewBookflow()
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
		_, _ = bookflow.Read(make([]byte, 4096))

		Convey("It should classify spoof trap", func() {
			So(bookflow.outcome.Category, ShouldEqual, 2)
		})
	})
}

func TestBookflowSpoofContrast(testingTB *testing.T) {
	Convey("Given weighted and touch histories", testingTB, func() {
		weighted := []float64{0.6, 0.55, 0.58, 0.62}
		level1 := []float64{0.2, 0.18, 0.22, 0.19}

		Convey("It should derive spoof contrast from medians", func() {
			contrast := bookflowSpoofContrast(weighted, level1)

			So(contrast, ShouldBeGreaterThan, 0)
			So(contrast, ShouldBeLessThan, 1)
		})
	})
}

func TestBookflowThinningGate(testingTB *testing.T) {
	Convey("Given weighted and flat histories", testingTB, func() {
		weighted := []float64{0.6, 0.55, 0.58, 0.62}
		flat := []float64{0.2, 0.18, 0.22, 0.19}

		Convey("It should derive thinning gate from medians", func() {
			gate := bookflowThinningGate(weighted, flat)

			So(gate, ShouldBeGreaterThan, 0)
			So(gate, ShouldBeLessThan, 1)
		})
	})
}

func BenchmarkBookflowRead(b *testing.B) {
	bookflow := NewBookflow()
	samples := []float64{
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
		_ = tests.WriteSamples(bookflow, samples...)
		_, _ = bookflow.Read(make([]byte, 4096))
	}
}
