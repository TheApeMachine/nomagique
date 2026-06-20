package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestRollingZScoreRead(t *testing.T) {
	Convey("Given a rolling z-score stage", t, func() {
		config := datura.Acquire("rolling-zscore-config", datura.APPJSON).
			Poke([]float64{-0.01, 0.0, 0.01, 0.02}, "state", "returns")

		stage := NewRollingZScore(config)
		artifact := datura.Acquire("rolling-zscore-test", datura.APPJSON).Poke(0.03, "sample")

		err := transport.NewFlipFlop(artifact, stage)

		Convey("It should normalize the current sample", func() {
			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "sample"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkRollingZScoreRead(b *testing.B) {
	config := datura.Acquire("rolling-zscore-bench", datura.APPJSON).
		Poke([]float64{-0.01, 0.0, 0.01, 0.02}, "state", "returns")

	stage := NewRollingZScore(config)
	artifact := datura.Acquire("rolling-zscore-bench-test", datura.APPJSON).Poke(0.03, "sample")

	b.ReportAllocs()

	for b.Loop() {
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
