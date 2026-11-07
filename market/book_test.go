package market

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	. "github.com/smartystreets/goconvey/convey"
)

func TestBookTouchAndChecksum(t *testing.T) {
	Convey("Given a book with one bid and one ask order", t, func() {
		book := NewBook()
		book.NoBookCrossing = false
		book.EnableMaxDepth = false

		bidPx := decimal.RequireFromString("100")
		bidQty := decimal.RequireFromString("2")
		askPx := decimal.RequireFromString("101")
		askQty := decimal.RequireFromString("3")
		now := time.Now()

		book.Update(&Delta{
			Direction: SideBid,
			ID:        "b1",
			Price:     &bidPx,
			Quantity:  &bidQty,
			Timestamp: now,
		})
		book.Update(&Delta{
			Direction: SideAsk,
			ID:        "a1",
			Price:     &askPx,
			Quantity:  &askQty,
			Timestamp: now,
		})

		Convey("BestBid and BestAsk reflect the resting touch", func() {
			So(book.BestBid(), ShouldNotBeNil)
			So(book.BestAsk(), ShouldNotBeNil)
			So(book.BestBid().Price.String(), ShouldEqual, "100")
			So(book.BestAsk().Price.String(), ShouldEqual, "101")
		})

		Convey("Midpoint and Spread are derived from the touch", func() {
			So(book.Midpoint().String(), ShouldEqual, "100.5")
			So(book.Spread().InexactFloat64(), ShouldBeBetween, 0.9, 1.0)
		})

		Convey("L3Checksum produces a stable local checksum", func() {
			result := book.L3Checksum("0")
			So(result.LocalChecksum, ShouldNotBeEmpty)
			So(result.Match, ShouldBeFalse)
		})
	})
}
