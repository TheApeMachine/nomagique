package statistic_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/statistic"
)

func TestIntegration(t *testing.T) {
	Convey("Given statistic stages composed through nomagique.Number", t, func() {
		Convey("When Panel registers peers before Median excludes the caller", func() {
			artifact := datura.Acquire("test", datura.APPJSON)
			crossSection := nomagique.Number(statistic.NewPanel(), statistic.NewMedian())

			for _, member := range []struct {
				key   float64
				value float64
			}{
				{1, 0.02},
				{2, 0.04},
				{3, 0.06},
			} {
				artifact.Poke(member.key, "member").Poke(member.value, "sample")
				err := transport.NewFlipFlop(artifact, crossSection)

				So(err, ShouldBeNil)
			}

			artifact.Poke(1, "member").Poke(0.02, "sample")
			err := transport.NewFlipFlop(artifact, crossSection)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0.05)
		})

		Convey("When Mean streams a uniform series", func() {
			artifact := datura.Acquire("test", datura.APPJSON)
			mean := statistic.NewMean()

			for _, sample := range []float64{1, 2, 3, 4} {
				artifact.Poke(sample, "sample")
				err := transport.NewFlipFlop(artifact, mean)

				So(err, ShouldBeNil)
			}

			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 2.5)
		})
	})
}
