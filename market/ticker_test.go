package market

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTickerMergeViaToMarket(t *testing.T) {
	Convey("ToMarket merges rows without wiping prior symbols", t, func() {
		book := NewTicker(map[string]*TickerData{
			"AAA/USD": {Symbol: "AAA/USD", Last: 1},
		})
		batch := NewTicker(map[string]*TickerData{
			"BBB/USD": {Symbol: "BBB/USD", Last: 2, VolumeUSD: 50},
		})

		batch.ToMarket(book)

		So(book.Of("AAA/USD").Last, ShouldEqual, 1)
		So(book.Of("BBB/USD").Last, ShouldEqual, 2)
		So(book.VolumeUSD("BBB/USD"), ShouldEqual, 50)
	})
}
