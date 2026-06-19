package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestMinSeries(t *testing.T) {
	Convey("Given a Min stage", t, func() {
		minStage := NewMin()
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, sample := range []float64{3, 1, 2} {
			artifact.Poke(sample, "sample")
			err := transport.NewFlipFlop(artifact, minStage)

			So(err, ShouldBeNil)
		}

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return the minimum", func() {
			So(got, ShouldEqual, 1)
		})
	})
}
