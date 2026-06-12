package vector

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

const (
	testInputBidPrice = iota
	testInputAskPrice
	testInputBidQty
	testInputAskQty
	testL1InputCount
)

const (
	testFeatureMidPrice = iota
	testFeatureSpreadBPS
	testFeatureImbalance
)

func newTestL1Extractor() (*FeatureExtractor, error) {
	return NewFeatureExtractor(testL1InputCount,
		func(inputs []float64) float64 {
			return (inputs[testInputBidPrice] + inputs[testInputAskPrice]) / 2
		},
		func(inputs []float64) float64 {
			mid := (inputs[testInputBidPrice] + inputs[testInputAskPrice]) / 2

			if mid <= 0 {
				return 0
			}

			return (inputs[testInputAskPrice] - inputs[testInputBidPrice]) / mid * 10000
		},
		func(inputs []float64) float64 {
			total := inputs[testInputBidQty] + inputs[testInputAskQty]

			if total <= 0 {
				return 0
			}

			return (inputs[testInputBidQty] - inputs[testInputAskQty]) / total
		},
	)

}

func TestFeatureExtractorExtract(testingTB *testing.T) {
	Convey("Given an L1 feature extractor", testingTB, func() {
		extractor, err := newTestL1Extractor()
		So(err, ShouldBeNil)

		Convey("It should derive mid, spread, and imbalance", func() {
			So(extractor.SetInput(testInputBidPrice, 100), ShouldBeNil)
			So(extractor.SetInput(testInputAskPrice, 101), ShouldBeNil)
			So(extractor.SetInput(testInputBidQty, 3), ShouldBeNil)
			So(extractor.SetInput(testInputAskQty, 1), ShouldBeNil)

			features := extractor.Extract()

			So(features[testFeatureMidPrice], ShouldAlmostEqual, 100.5, 1e-9)
			So(features[testFeatureSpreadBPS], ShouldBeGreaterThan, 0)
			So(features[testFeatureImbalance], ShouldAlmostEqual, 0.5, 1e-9)
		})
	})
}

func TestFeatureNodeObserve(testingTB *testing.T) {
	Convey("Given a feature node", testingTB, func() {
		extractor, err := newTestL1Extractor()
		So(err, ShouldBeNil)
		node := NewFeatureNode(extractor, testFeatureSpreadBPS)

		_ = extractor.SetInput(testInputBidPrice, 100)
		_ = extractor.SetInput(testInputAskPrice, 101)
		_ = extractor.SetInput(testInputBidQty, 1)
		_ = extractor.SetInput(testInputAskQty, 1)
		extractor.Extract()

		Convey("It should expose the derived spread", func() {
			So(float64(node.Observe()), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkFeatureExtractorExtract(testingTB *testing.B) {
	extractor, err := newTestL1Extractor()

	if err != nil {
		testingTB.Fatal(err)
	}

	_ = extractor.SetInput(testInputBidPrice, 100)
	_ = extractor.SetInput(testInputAskPrice, 101)
	_ = extractor.SetInput(testInputBidQty, 3)
	_ = extractor.SetInput(testInputAskQty, 1)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		extractor.Extract()
	}
}
