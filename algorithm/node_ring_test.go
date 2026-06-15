package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewNodeRing(testingTB *testing.T) {
	Convey("Given NewNodeRing", testingTB, func() {
		nodeRing := NewNodeRing(4, 8)

		Convey("It should start empty", func() {
			So(nodeRing, ShouldNotBeNil)
			So(nodeRing.AlignedLength(), ShouldEqual, 0)
		})
	})
}

func TestNodeRing_Read(testingTB *testing.T) {
	Convey("Given aligned node observations", testingTB, func() {
		nodeRing := NewNodeRing(4, 8)

		for index := range 16 {
			observeNodeRow(
				nodeRing,
				float64(index)*0.1,
				float64(index)*0.2,
				float64(index)*0.5,
				float64(index)*0.05,
			)
		}

		Convey("It should retain bounded aligned history", func() {
			So(nodeRing.AlignedLength(), ShouldEqual, 8)
			So(len(nodeRing.Streams()[3]), ShouldEqual, 8)
		})
	})

	Convey("Given partial node inputs", testingTB, func() {
		nodeRing := NewNodeRing(4, 8)
		before := observeInputs(nodeRing, 1)

		Convey("It should ignore misaligned rows", func() {
			So(before, ShouldEqual, 0)
			So(nodeRing.AlignedLength(), ShouldEqual, 0)
		})
	})
}

func BenchmarkNodeRing_Read(b *testing.B) {
	nodeRing := NewNodeRing(4, 16)

	b.ReportAllocs()

	for b.Loop() {
		observeNodeRow(nodeRing, 0.1, 0.2, 0.5, 0.05)
	}
}
