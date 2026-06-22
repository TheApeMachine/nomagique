package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEvidenceGeomean(testingTB *testing.T) {
	Convey("Given positive evidence margins", testingTB, func() {
		Convey("It should return their geometric mean", func() {
			mean, err := EvidenceGeomean(0.5, 0.5)

			So(err, ShouldBeNil)
			So(mean, ShouldAlmostEqual, 0.5, 1e-9)
		})
	})

	Convey("Given a non-positive margin", testingTB, func() {
		Convey("It should return an error", func() {
			_, err := EvidenceGeomean(0.5, 0)

			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given no margins", testingTB, func() {
		Convey("It should return an error", func() {
			_, err := EvidenceGeomean()

			So(err, ShouldNotBeNil)
		})
	})
}
