package hawkes

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBivariateFit_ImmediateOffspring(t *testing.T) {
	Convey("Given a valid subcritical branching matrix", t, func() {
		fit := offspringFit()
		buy, sell, ok := fit.ImmediateOffspring()

		Convey("It should sum first-generation children by parent side", func() {
			So(ok, ShouldBeTrue)
			So(buy, ShouldAlmostEqual, 0.5, 1e-12)
			So(sell, ShouldAlmostEqual, 0.5, 1e-12)
		})
	})
}

func TestBivariateFit_TotalDescendants(t *testing.T) {
	Convey("Given a valid subcritical branching matrix", t, func() {
		fit := offspringFit()
		buy, sell, ok := fit.TotalDescendants()

		Convey("It should sum every generation without counting the parent", func() {
			So(ok, ShouldBeTrue)
			So(buy, ShouldAlmostEqual, 1.0, 1e-12)
			So(sell, ShouldAlmostEqual, 1.0, 1e-12)
		})
	})

	Convey("Given a critical fit", t, func() {
		fit := offspringFit()
		fit.SpectralRadius = 1
		_, _, ok := fit.TotalDescendants()

		Convey("It should reject an unbounded cascade expectation", func() {
			So(ok, ShouldBeFalse)
		})
	})
}

func offspringFit() BivariateFit {
	fit := BivariateFit{
		MuX:     1,
		MuY:     1,
		AlphaXX: 0.4,
		AlphaXY: 0.2,
		AlphaYX: 0.6,
		AlphaYY: 0.8,
		Beta:    2,
	}
	fit.SpectralRadius = fit.computeSpectralRadius()

	return fit
}
