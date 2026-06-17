package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEvidenceGeomean(testingTB *testing.T) {
	Convey("Given positive evidence margins", testingTB, func() {
		Convey("It should return their geometric mean", func() {
			So(EvidenceGeomean(0.5, 0.5), ShouldAlmostEqual, 0.5, 1e-9)
		})
	})

	Convey("Given a non-positive margin", testingTB, func() {
		Convey("It should return zero", func() {
			So(EvidenceGeomean(0.5, 0), ShouldEqual, 0)
		})
	})
}
