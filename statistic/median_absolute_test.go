package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestMedianAbsoluteSeries(t *testing.T) {
	Convey("Given a MedianAbsolute stage", t, func() {
		medianAbsolute := NewMedianAbsolute(datura.Acquire("median-absolute-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, sample := range []float64{-1, 2, -3} {
			artifact.Poke(sample, "sample")
			err := transport.NewFlipFlop(artifact, medianAbsolute)

			So(err, ShouldBeNil)
		}

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should ignore sign", func() {
			So(got, ShouldEqual, 2)
		})
	})
}
