package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestMaxSeries(t *testing.T) {
	Convey("Given a Max stage", t, func() {
		maxStage := NewMax()
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, sample := range []float64{3, 1, 2} {
			artifact.Poke(sample, "sample")
			err := transport.NewFlipFlop(artifact, maxStage)
			So(err, ShouldBeNil)
		}

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should retain the maximum", func() {
			So(got, ShouldEqual, 3)
		})
	})
}
