package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func TestMedianAbsoluteRead(t *testing.T) {
	Convey("Given a MedianAbsolute stage", t, func() {
		medianAbsolute := NewMedianAbsolute(scalarStageConfig("median-absolute-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, sample := range []float64{-1, 2, -3} {
			err := nomagique.RoundTripArtifact(ScalarWire(artifact, "sample", sample), medianAbsolute)

			So(err, ShouldBeNil)
		}

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should ignore sign", func() {
			So(got, ShouldEqual, 2)
		})
	})
}
