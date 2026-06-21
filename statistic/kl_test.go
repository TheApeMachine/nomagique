package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestKLDivergenceSeries(t *testing.T) {
	Convey("Given a KL stage", t, func() {
		kl := NewKLDivergence(0, 0)
		artifact := datura.Acquire("test", datura.APPJSON)
		observed := []float64{1, 1, 1, 1}
		expected := []float64{1, 1, 1, 1}

		for index := 0; index < len(observed); index++ {
			artifact.Poke(observed[index], "sample").Poke(expected[index], "paired")
			err := transport.NewFlipFlop(artifact, kl)

			So(err, ShouldBeNil)
		}

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return zero divergence for identical mass", func() {
			So(got, ShouldAlmostEqual, 0, 1e-9)
		})
	})
}
