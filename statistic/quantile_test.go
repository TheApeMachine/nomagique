package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"gonum.org/v1/gonum/stat"
)

func TestQuantileRead(t *testing.T) {
	Convey("Given a Quantile stage", t, func() {
		quantileConfig := datura.Acquire("quantile-config", datura.APPJSON).
			Poke("sample", "input").
			Poke("value", "outputKey").
			Poke(0.5, "config", "percentile").
			Poke(float64(stat.LinInterp), "config", "kind")
		quantile := NewQuantile(quantileConfig)
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, sample := range []float64{1, 2, 3, 4} {
			err := transport.NewFlipFlop(ScalarWire(artifact, "sample", sample), quantile)

			So(err, ShouldBeNil)
		}

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return the expected quantile", func() {
			So(got, ShouldEqual, 2)
		})
	})
}
