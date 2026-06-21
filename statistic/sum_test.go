package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestSumSeries(t *testing.T) {
	Convey("Given a Sum stage", t, func() {
		sum := NewSum()
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, sample := range []float64{1, 2, 3, 4} {
			artifact.Poke(sample, "sample")
			err := transport.NewFlipFlop(artifact, sum)

			So(err, ShouldBeNil)
		}

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should accumulate the series", func() {
			So(got, ShouldEqual, 10)
		})
	})
}

func BenchmarkSumRead(b *testing.B) {
	sum := NewSum()
	artifact := datura.Acquire("test", datura.APPJSON)

	b.ReportAllocs()

	for b.Loop() {
		artifact.Poke(1, "sample")
		_ = transport.NewFlipFlop(artifact, sum)
	}
}
