package market

import (
	"testing"

	"github.com/shopspring/decimal"
	. "github.com/smartystreets/goconvey/convey"
)

func TestCommitSpend(t *testing.T) {
	Convey("CommitSpend", t, func() {
		balance := NewBalance(map[string]any{
			"quote": "USD",
			"assets": []*Asset{
				NewAsset(nil, "USD", decimal.NewFromInt(1000), decimal.Zero, decimal.Zero),
			},
		})

		Convey("Applies fill to Assets and clears the matching Reserve", func() {
			balance.Reserve("USD", decimal.NewFromInt(250))
			So(balance.Cash("USD").Equal(decimal.NewFromInt(750)), ShouldBeTrue)

			balance.CommitSpend("USD", decimal.NewFromInt(250))

			So(balance.Assets[0].Capital.Equal(decimal.NewFromInt(750)), ShouldBeTrue)
			So(balance.Cash("USD").Equal(decimal.NewFromInt(750)), ShouldBeTrue)
		})

		Convey("Leaves other quote reservations intact", func() {
			balance.Reserve("USD", decimal.NewFromInt(100))
			balance.Reserve("USD", decimal.NewFromInt(50))
			balance.CommitSpend("USD", decimal.NewFromInt(100))

			So(balance.Cash("USD").Equal(decimal.NewFromInt(850)), ShouldBeTrue)
		})
	})
}
