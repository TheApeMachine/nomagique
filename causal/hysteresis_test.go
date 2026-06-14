package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestDeriveRegimeHysteresisSamples(testingTB *testing.T) {
	Convey("Given history length", testingTB, func() {
		samples := DeriveRegimeHysteresisSamples(100)

		Convey("It should return a positive window", func() {
			So(samples, ShouldBeGreaterThan, 0)
		})
	})
}
