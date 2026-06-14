package vector

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
)

func TestInputSlotObserve(testingTB *testing.T) {
	Convey("Given an input slot on an L1 extractor", testingTB, func() {
		extractor, err := NewL1BookExtractor()
		So(err, ShouldBeNil)

		bidSlot, err := NewInputSlot(extractor, L1BidPrice)
		So(err, ShouldBeNil)

		Convey("It should store the observed sample on the bound channel", func() {
			So(bidSlot.Observe(nomagique.Scalar(100)), ShouldEqual, 100)

			bid, inputErr := extractor.Input(L1BidPrice)
			So(inputErr, ShouldBeNil)
			So(bid, ShouldEqual, 100)
		})
	})
}

func TestNewInputSlotInvalidChannel(testingTB *testing.T) {
	Convey("Given an invalid channel index", testingTB, func() {
		extractor, err := NewL1BookExtractor()
		So(err, ShouldBeNil)

		_, slotErr := NewInputSlot(extractor, L1InputCount)

		Convey("It should return an error", func() {
			So(slotErr, ShouldNotBeNil)
		})
	})
}

func BenchmarkInputSlotObserve(testingTB *testing.B) {
	extractor, err := NewL1BookExtractor()

	if err != nil {
		testingTB.Fatal(err)
	}

	bidSlot, err := NewInputSlot(extractor, L1BidPrice)

	if err != nil {
		testingTB.Fatal(err)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = bidSlot.Observe(nomagique.Scalar(100))
	}
}
