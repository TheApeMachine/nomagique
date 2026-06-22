package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestStdDevSeries(t *testing.T) {
	Convey("Given a StdDev stage", t, func() {
		stdDev := NewStdDev(datura.Acquire("stddev-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)
		var got float64

		for _, sample := range []float64{1, 2, 3, 4} {
			artifact.Poke(sample, "sample")
			err := transport.NewFlipFlop(artifact, stdDev)

			if err != nil {
				continue
			}

			got = datura.Peek[float64](artifact, "output", "value")
		}

		Convey("It should derive dispersion from history", func() {
			So(got, ShouldBeGreaterThan, 0)
		})
	})
}
