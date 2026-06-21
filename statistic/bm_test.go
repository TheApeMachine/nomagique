package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestBivariateMomentSeries(t *testing.T) {
	Convey("Given a BivariateMoment stage", t, func() {
		bivariateConfig := datura.Acquire("bm-config", datura.APPJSON).
			Poke(1.0, "config", "r").
			Poke(1.0, "config", "s")
		bivariateMoment := NewBivariateMoment(bivariateConfig)
		artifact := datura.Acquire("test", datura.APPJSON)
		xValues := []float64{1, 2, 3, 4}
		yValues := []float64{2, 5, 7, 10}

		for index := 0; index < len(xValues); index++ {
			artifact.Poke(xValues[index], "sample").Poke(yValues[index], "paired")
			err := transport.NewFlipFlop(artifact, bivariateMoment)

			So(err, ShouldBeNil)
		}

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should compute the mixed moment", func() {
			So(got, ShouldBeGreaterThan, 0)
		})
	})
}
