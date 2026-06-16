package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/tests"
)

func TestDepthEvaluate(testingTB *testing.T) {
	Convey("Given deep quote volume versus peers", testingTB, func() {
		depth := NewDepth()
		writeErr := tests.WriteSamples(depth,
			1200, 4,
			800, 900, 1000, 1100,
			1, 0,
		)
		So(writeErr, ShouldBeNil)
		_, _ = depth.Read(make([]byte, 4096))

		Convey("It should classify robust liquidity", func() {
			So(depth.outcome.Eligible, ShouldBeTrue)
			So(depth.outcome.Category, ShouldEqual, 3)
		})
	})

	Convey("Given peak scarcity volume", testingTB, func() {
		depth := NewDepth()
		writeErr := tests.WriteSamples(depth,
			50, 3,
			1100, 950, 50,
			1, 0,
		)
		So(writeErr, ShouldBeNil)
		_, _ = depth.Read(make([]byte, 4096))

		Convey("It should classify extreme scarcity", func() {
			So(depth.outcome.Category, ShouldEqual, 1)
		})
	})
}

func TestClassifyDepth(testingTB *testing.T) {
	Convey("Given peer quote volumes", testingTB, func() {
		peers := []float64{800, 900, 1000, 1100}
		sortedPeers := append([]float64(nil), peers...)

		Convey("It should map peer quartiles onto scarcity categories", func() {
			So(classifyDepth(1200, 875, 1075, false, false), ShouldEqual, 3)
			So(classifyDepth(950, 875, 1075, false, false), ShouldEqual, 2)
			So(classifyDepth(500, 875, 1075, true, false), ShouldEqual, 1)
			So(classifyDepth(500, 875, 1075, true, true), ShouldEqual, 2)
			So(len(sortedPeers), ShouldEqual, 4)
		})
	})
}

func TestAbsoluteScaledVolumes(testingTB *testing.T) {
	Convey("Given baseline-relative volume above history", testingTB, func() {
		scaledQuote, scaledPeers := AbsoluteScaledVolumes(
			300,
			[]float64{600, 700},
			3,
			true,
		)

		Convey("It should lift cross-section volumes before quartiles", func() {
			So(scaledQuote, ShouldEqual, 900)
			So(scaledPeers, ShouldResemble, []float64{1800, 2100})
		})
	})
}

func BenchmarkDepthRead(b *testing.B) {
	depth := NewDepth()
	samples := []float64{
		1200, 4,
		800, 900, 1000, 1100,
		1, 0,
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = tests.WriteSamples(depth, samples...)
		_, _ = depth.Read(make([]byte, 4096))
	}
}
