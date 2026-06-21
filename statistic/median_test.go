package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestMedianSeries(t *testing.T) {
	Convey("Given a Median stage", t, func() {
		median := NewMedian(datura.Acquire("median-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, sample := range []float64{3, 1, 2} {
			artifact.Poke(sample, "sample")
			err := transport.NewFlipFlop(artifact, median)

			So(err, ShouldBeNil)
		}

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return the expected median", func() {
			So(got, ShouldEqual, 2)
		})
	})
}

func TestMedianOf(t *testing.T) {
	Convey("Given unsorted values", t, func() {
		Convey("It should return the median", func() {
			So(MedianOf([]float64{3, 1, 2}), ShouldEqual, 2)
		})
	})
}
