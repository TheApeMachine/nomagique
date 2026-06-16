package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/tests"
)

func TestBookQualityToxicBluff(testingTB *testing.T) {
	Convey("Given near-touch toxic churn above gate", testingTB, func() {
		bookQuality := NewBookQuality()
		writeErr := tests.WriteSamples(bookQuality,
			0, 0.1, 0, 0.1,
			80, 80,
			1, 4.5,
			0.15, 0.8, 0, 2,
			100,
		)
		So(writeErr, ShouldBeNil)
		_, _ = bookQuality.Read(make([]byte, 4096))

		Convey("It should classify toxic bluff", func() {
			So(bookQuality.outcome.Eligible, ShouldBeTrue)
			So(bookQuality.outcome.Category, ShouldEqual, 1)
			So(bookQuality.outcome.Strength, ShouldEqual, 4.5)
		})
	})
}

func TestBookQualityLiquidityVacuum(testingTB *testing.T) {
	Convey("Given cancel/fill asymmetry with fill flow", testingTB, func() {
		bookQuality := NewBookQuality()
		writeErr := tests.WriteSamples(bookQuality,
			0.3, 0.1, 0, 0,
			10, 10,
			0, 0,
			0.15, 0, 0, 2,
			50000,
		)
		So(writeErr, ShouldBeNil)
		_, _ = bookQuality.Read(make([]byte, 4096))

		Convey("It should classify liquidity vacuum", func() {
			So(bookQuality.outcome.Category, ShouldEqual, 2)
			So(bookQuality.outcome.Strength, ShouldBeGreaterThan, 0)
		})
	})
}

func TestBookQualityHardSupport(testingTB *testing.T) {
	Convey("Given balanced depth with fills and no cancels", testingTB, func() {
		bookQuality := NewBookQuality()
		writeErr := tests.WriteSamples(bookQuality,
			0, 0.1, 0, 0.1,
			80, 80,
			0, 0,
			0.15, 0, 0, 1,
			100,
		)
		So(writeErr, ShouldBeNil)
		_, _ = bookQuality.Read(make([]byte, 4096))

		Convey("It should classify hard support", func() {
			So(bookQuality.outcome.Category, ShouldEqual, 3)
			So(bookQuality.outcome.Strength, ShouldEqual, 1)
		})
	})
}

func BenchmarkBookQualityRead(b *testing.B) {
	bookQuality := NewBookQuality()
	samples := []float64{
		0.3, 0.1, 0, 0,
		10, 10,
		0, 0,
		0.15, 0, 0, 2,
		50000,
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = tests.WriteSamples(bookQuality, samples...)
		_, _ = bookQuality.Read(make([]byte, 4096))
	}
}
