package market

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/shopspring/decimal"
)

func TestBalanceReading(t *testing.T) {
	Convey("Given a Balance with quote capital", t, func() {
		usd := decimal.RequireFromString("160.14")
		equity := decimal.RequireFromString("199.68")
		unrealized := decimal.RequireFromString("-0.32")

		balance := NewBalance("USD")
		balance.Replace([]*Asset{
			NewAsset(nil, "USD", usd, unrealized, equity),
			NewAsset(nil, "ZAMA", decimal.RequireFromString("517.71"), decimal.Zero, decimal.RequireFromString("517.71")),
		})

		Convey("It exposes the dashboard reading map", func() {
			reading := balance.Reading()
			So(reading, ShouldNotBeNil)
			So(reading["cash"].(*decimal.Decimal).String(), ShouldEqual, usd.String())
			So(reading["equity"].(*decimal.Decimal).String(), ShouldEqual, equity.String())
			So(reading["unrealized"].(*decimal.Decimal).String(), ShouldEqual, unrealized.String())
		})

		Convey("Cash resolves venue aliases", func() {
			So(balance.Cash("ZUSD").String(), ShouldEqual, usd.String())
		})
	})
}
