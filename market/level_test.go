package market

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	. "github.com/smartystreets/goconvey/convey"
)

func TestLevelUpdateCancel(t *testing.T) {
	Convey("a zero-qty delta removes the order without double-counting", t, func() {
		level := NewLevel()
		px := decimal.RequireFromString("100")
		qty := decimal.RequireFromString("5")

		level.update(&Delta{
			ID: "o1", Price: &px, Quantity: &qty, Timestamp: time.Unix(1, 0),
		})
		So(level.Quantity.String(), ShouldEqual, "5")

		zero := decimal.Zero
		level.update(&Delta{
			ID: "o1", Price: &px, Quantity: &zero, Timestamp: time.Unix(2, 0),
		})

		So(level.orders["o1"], ShouldBeNil)
		So(level.Quantity.IsZero(), ShouldBeTrue)
	})

	Convey("a modify replaces size instead of subtracting a negative delta", t, func() {
		level := NewLevel()
		px := decimal.RequireFromString("100")
		qty := decimal.RequireFromString("10")

		level.update(&Delta{
			ID: "o1", Price: &px, Quantity: &qty, Timestamp: time.Unix(1, 0),
		})

		next := decimal.RequireFromString("3")
		level.update(&Delta{
			ID: "o1", Price: &px, Quantity: &next, Timestamp: time.Unix(2, 0),
		})

		So(level.Quantity.String(), ShouldEqual, "3")
	})
}
