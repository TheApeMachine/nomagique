package market

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	. "github.com/smartystreets/goconvey/convey"
)

func TestL3Checksum(t *testing.T) {
	Convey("Given resting L3 orders scaled to pair/lot decimals", t, func() {
		book := NewBook()
		book.NoBookCrossing = false
		book.EnableMaxDepth = false
		now := time.Now()

		// Trailing zeros that Decimal.String() would drop — Kraken still
		// includes them in the CRC (lot_decimals=5, pair_decimals=4).
		askPx := decimal.RequireFromString("0.1673")
		askQty := decimal.RequireFromString((decimal.RequireFromString("611.37336")).StringFixed(5))
		bidPx := decimal.RequireFromString((decimal.RequireFromString("0.1672")).StringFixed(4))
		bidQty := decimal.RequireFromString((decimal.RequireFromString("874.21637")).StringFixed(5))

		So(askQty.Exponent(), ShouldEqual, -5)
		So(bidPx.Exponent(), ShouldEqual, -4)

		book.Update(&Delta{
			Direction: SideAsk, ID: "a1", Price: &askPx, Quantity: &askQty, Timestamp: now,
		})
		book.Update(&Delta{
			Direction: SideBid, ID: "b1", Price: &bidPx, Quantity: &bidQty, Timestamp: now,
		})

		Convey("When L3Checksum tokens keep fixed-scale trailing zeros", func() {
			result := book.L3Checksum("0")

			So(result.Asks, ShouldEqual, "167361137336")
			So(result.Bids, ShouldEqual, "167287421637")
		})

		Convey("When String-stripped quantities would diverge from the venue", func() {
			stripped := decimal.RequireFromString("611.37336")
			So(stripped.Exponent(), ShouldEqual, -5)
			// Same magnitude, but if Exp were shorter the token would lose a zero.
			short := decimal.RequireFromString("2641.6312")
			padded := decimal.RequireFromString(short.StringFixed(5))
			So(checksumToken(&short), ShouldEqual, "26416312")
			So(checksumToken(&padded), ShouldEqual, "264163120")
			So(checksumToken(&short), ShouldNotEqual, checksumToken(&padded))
		})
	})
}
