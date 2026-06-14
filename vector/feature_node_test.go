package vector

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
)

func TestFeatureNodeObserve(testingTB *testing.T) {
	Convey("Given a spread feature node", testingTB, func() {
		extractor, err := NewL1BookExtractor()
		So(err, ShouldBeNil)

		spreadNode, err := NewFeatureNode(extractor, L1SpreadBPS)
		So(err, ShouldBeNil)

		_ = extractor.SetInput(L1BidPrice, 100)
		_ = extractor.SetInput(L1AskPrice, 101)
		_ = extractor.SetInput(L1BidQty, 1)
		_ = extractor.SetInput(L1AskQty, 1)

		Convey("It should expose spread after Observe runs Extract", func() {
			So(spreadNode.Observe(), ShouldBeGreaterThan, 0)
		})
	})
}

func TestFeatureNodeNumber(testingTB *testing.T) {
	Convey("Given a composed spread number", testingTB, func() {
		extractor, err := NewL1BookExtractor()
		So(err, ShouldBeNil)

		spreadNode, err := NewFeatureNode(extractor, L1SpreadBPS)
		So(err, ShouldBeNil)

		spreadNumber, err := nomagique.Number(spreadNode)
		So(err, ShouldBeNil)

		_ = extractor.SetInput(L1BidPrice, 100)
		_ = extractor.SetInput(L1AskPrice, 101)
		_ = extractor.SetInput(L1BidQty, 1)
		_ = extractor.SetInput(L1AskQty, 1)

		Convey("It should observe through the registered pipeline", func() {
			So(nomagique.Scalar(0).Observe(spreadNumber), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkFeatureNodeObserve(testingTB *testing.B) {
	extractor, err := NewL1BookExtractor()

	if err != nil {
		testingTB.Fatal(err)
	}

	spreadNode, err := NewFeatureNode(extractor, L1SpreadBPS)

	if err != nil {
		testingTB.Fatal(err)
	}

	_ = extractor.SetInput(L1BidPrice, 100)
	_ = extractor.SetInput(L1AskPrice, 101)
	_ = extractor.SetInput(L1BidQty, 1)
	_ = extractor.SetInput(L1AskQty, 1)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = spreadNode.Observe()
	}
}
