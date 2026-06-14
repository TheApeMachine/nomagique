package vector

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFeatureExtractorExtract(testingTB *testing.T) {
	Convey("Given an L1 feature extractor", testingTB, func() {
		extractor, err := NewL1BookExtractor()
		So(err, ShouldBeNil)

		Convey("It should derive mid, spread, and imbalance", func() {
			So(extractor.SetInput(L1BidPrice, 100), ShouldBeNil)
			So(extractor.SetInput(L1AskPrice, 101), ShouldBeNil)
			So(extractor.SetInput(L1BidQty, 3), ShouldBeNil)
			So(extractor.SetInput(L1AskQty, 1), ShouldBeNil)

			features := extractor.Extract()

			So(features[L1MidPrice], ShouldAlmostEqual, 100.5, 1e-9)
			So(features[L1SpreadBPS], ShouldBeGreaterThan, 0)
			So(features[L1Imbalance], ShouldAlmostEqual, 0.5, 1e-9)
		})
	})
}

func TestFeatureExtractorReset(testingTB *testing.T) {
	Convey("Given a populated extractor", testingTB, func() {
		extractor, err := NewL1BookExtractor()
		So(err, ShouldBeNil)

		_ = extractor.SetInput(L1BidPrice, 100)
		extractor.Extract()
		So(extractor.Reset(), ShouldBeNil)

		bid, inputErr := extractor.Input(L1BidPrice)
		So(inputErr, ShouldBeNil)
		So(bid, ShouldEqual, 0)
	})
}

func BenchmarkFeatureExtractorExtract(testingTB *testing.B) {
	extractor, err := NewL1BookExtractor()

	if err != nil {
		testingTB.Fatal(err)
	}

	_ = extractor.SetInput(L1BidPrice, 100)
	_ = extractor.SetInput(L1AskPrice, 101)
	_ = extractor.SetInput(L1BidQty, 3)
	_ = extractor.SetInput(L1AskQty, 1)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		extractor.Extract()
	}
}
