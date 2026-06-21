package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func precursorConfig() *datura.Artifact {
	return datura.Acquire("precursor-config", datura.APPJSON).
		Poke([]string{"rvol", "precursor"}, "order").
		Poke(1.0, "stageIndex").
		Poke(map[string]any{
			"precursor": map[string]any{
				"input":      "last",
				"returnLag":  1.0,
				"longWindow": 5.0,
				"outputKey":  "precursor",
				"stageIndex": 1.0,
			},
		}, "inputs")
}

func precursorState(last float64) *datura.Artifact {
	artifact := datura.Acquire("precursor-state", datura.APPJSON)
	artifact.Merge("root", "features")
	artifact.Merge("inputs", []string{"volume", "last"})
	artifact.Merge("features", []float64{100, last})

	return artifact
}

func TestPriceRingRead(t *testing.T) {
	Convey("Given a price ring stage", t, func() {
		config := precursorConfig()
		stage := NewPriceRing(config)

		for _, last := range []float64{100, 101, 102} {
			artifact := precursorState(last)
			err := transport.NewFlipFlop(artifact, stage)
			So(err, ShouldBeNil)
		}

		Convey("It should retain trimmed prices on the config artifact", func() {
			prices := datura.Peek[[]float64](config, "prices")
			So(len(prices), ShouldEqual, 3)
			So(prices[len(prices)-1], ShouldEqual, 102)
		})
	})
}

func BenchmarkPriceRingRead(b *testing.B) {
	config := precursorConfig()
	stage := NewPriceRing(config)
	artifact := precursorState(101)

	b.ReportAllocs()

	for b.Loop() {
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
