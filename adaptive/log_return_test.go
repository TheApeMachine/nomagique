package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestLogReturnRead(t *testing.T) {
	Convey("Given a log-return stage with retained prices", t, func() {
		config := datura.Acquire("log-return-config", datura.APPJSON).
			Poke([]string{"rvol", "precursor"}, "order").
			Poke(1.0, "stageIndex").
			Poke(map[string]any{
				"precursor": map[string]any{
					"input":      "last",
					"returnLag":  1.0,
					"longWindow": 5.0,
					"outputKey":  "precursor",
				},
			}, "inputs")

		config.Merge("prices", []float64{100, 101})

		stage := NewLogReturn(config)
		artifact := datura.Acquire("log-return-test", datura.APPJSON).Poke(102.0, "sample")

		err := transport.NewFlipFlop(artifact, stage)

		Convey("It should append a log return to the rolling series", func() {
			So(err, ShouldBeNil)
			So(len(datura.Peek[[]float64](config, "returns")), ShouldEqual, 1)
			So(datura.Peek[float64](artifact, "sample"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkLogReturnRead(b *testing.B) {
	config := datura.Acquire("log-return-bench", datura.APPJSON).
		Poke([]string{"rvol", "precursor"}, "order").
		Poke(map[string]any{
			"precursor": map[string]any{
				"returnLag":  1.0,
				"longWindow": 5.0,
			},
		}, "inputs").
		Poke([]float64{100, 101, 102}, "state", "prices")

	stage := NewLogReturn(config)
	artifact := datura.Acquire("log-return-bench-test", datura.APPJSON).Poke(103.0, "sample")

	b.ReportAllocs()

	for b.Loop() {
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
