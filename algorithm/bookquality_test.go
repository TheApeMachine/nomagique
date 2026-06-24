package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/tests"
)

func TestBookQualityToxicBluff(testingTB *testing.T) {
	Convey("Given near-touch toxic churn above gate", testingTB, func() {
		bookQuality := equation.NewBookQuality(equation.BookQualityConfig())
		err := tests.WriteSamples(bookQuality,
			0, 0.1, 0, 0.1,
			80, 80,
			1, 4.5,
			0.15, 0.8, 0, 2,
			100,
		)

		So(err, ShouldBeNil)

		outbound, err := readOutbound(bookQuality)

		So(err, ShouldBeNil)

		Convey("It should classify toxic bluff", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 4.5)
		})
	})
}

func TestBookQualityLiquidityVacuum(testingTB *testing.T) {
	Convey("Given cancel/fill asymmetry with fill flow", testingTB, func() {
		bookQuality := equation.NewBookQuality(equation.BookQualityConfig())
		err := tests.WriteSamples(bookQuality,
			0.3, 0.1, 0, 0,
			10, 10,
			0, 0,
			0.15, 0, 0, 2,
			50000,
		)

		So(err, ShouldBeNil)

		outbound, err := readOutbound(bookQuality)

		So(err, ShouldBeNil)

		Convey("It should classify liquidity vacuum", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 2)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})
}

func TestBookQualityHardSupport(testingTB *testing.T) {
	Convey("Given balanced depth with fills and no cancels", testingTB, func() {
		bookQuality := equation.NewBookQuality(equation.BookQualityConfig())
		err := tests.WriteSamples(bookQuality,
			0, 0.1, 0, 0.1,
			80, 80,
			0, 0,
			0.15, 0, 0, 1,
			100,
		)

		So(err, ShouldBeNil)

		outbound, err := readOutbound(bookQuality)

		So(err, ShouldBeNil)

		Convey("It should classify hard support", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 3)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 1)
		})
	})
}

func BenchmarkBookQualityRead(b *testing.B) {
	bookQuality := equation.NewBookQuality(equation.BookQualityConfig())
	samples := []float64{
		0.3, 0.1, 0, 0,
		10, 10,
		0, 0,
		0.15, 0, 0, 2,
		50000,
	}
	frame := make([]byte, 4096)

	b.ReportAllocs()

	for b.Loop() {
		_ = tests.WriteSamples(bookQuality, samples...)
		_, _ = bookQuality.Read(frame)
	}
}
