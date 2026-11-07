package store

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCatalog(t *testing.T) {
	Convey("Catalog covers every region key exactly once", t, func() {
		So(len(Catalog), ShouldEqual, RegionCount)

		seen := make([]bool, RegionCount)
		for _, regime := range Catalog {
			So(int(regime.Key), ShouldBeLessThan, RegionCount)
			So(seen[regime.Key], ShouldBeFalse)
			seen[regime.Key] = true
			So(regime.Category, ShouldNotBeEmpty)
			So(regime.Name(), ShouldEqual, regime.Category)
			So(regime.Kind == "action" || regime.Kind == "oracle", ShouldBeTrue)
		}

		for key, ok := range seen {
			So(ok, ShouldBeTrue)
			_ = key
		}
	})

	Convey("Pins invert Catalog without dropping envelope metrics", t, func() {
		pins := Catalog.Pins()

		So(pins["cvd"]["drive"], ShouldResemble, []uint8{0})
		So(pins["hawkes"]["spectral_radius"], ShouldResemble, []uint8{6, 18})
		So(pins["liquidity"]["scarcity_score"], ShouldResemble, []uint8{8, 21})

		So(Catalog.ByKey(0).Category, ShouldEqual, "ignition")
		So(Catalog.ByKey(6).Category, ShouldEqual, "excitation")
		So(Catalog.ByKey(18).Category, ShouldEqual, "board_conviction")
	})
}
