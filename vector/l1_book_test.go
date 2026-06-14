package vector

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewL1BookExtractor(t *testing.T) {
	Convey("Given an L1 book extractor", t, func() {
		extractor, err := NewL1BookExtractor()

		So(err, ShouldBeNil)

		Convey("When bid/ask touch and quantities are set", func() {
			So(extractor.SetInput(L1BidPrice, 100), ShouldBeNil)
			So(extractor.SetInput(L1AskPrice, 101), ShouldBeNil)
			So(extractor.SetInput(L1BidQty, 3), ShouldBeNil)
			So(extractor.SetInput(L1AskQty, 1), ShouldBeNil)
			extractor.Extract()

			mid, err := extractor.Feature(L1MidPrice)
			spreadBPS, spreadErr := extractor.Feature(L1SpreadBPS)
			imbalance, imbalanceErr := extractor.Feature(L1Imbalance)

			So(err, ShouldBeNil)
			So(spreadErr, ShouldBeNil)
			So(imbalanceErr, ShouldBeNil)
			So(mid, ShouldAlmostEqual, 100.5, 1e-9)
			So(spreadBPS, ShouldAlmostEqual, 99.50248756218905, 1e-6)
			So(imbalance, ShouldAlmostEqual, 0.5, 1e-9)
		})
	})
}

func BenchmarkNewL1BookExtractor(b *testing.B) {
	extractor, err := NewL1BookExtractor()

	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = extractor.SetInput(L1BidPrice, 100)
		_ = extractor.SetInput(L1AskPrice, 101)
		_ = extractor.SetInput(L1BidQty, 3)
		_ = extractor.SetInput(L1AskQty, 1)
		extractor.Extract()
		_, _ = extractor.Feature(L1MidPrice)
		_, _ = extractor.Feature(L1SpreadBPS)
		_, _ = extractor.Feature(L1Imbalance)
	}
}
