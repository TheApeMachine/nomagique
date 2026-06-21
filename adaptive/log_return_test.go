package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestLogReturnRead(t *testing.T) {
	Convey("Given a log-return stage fed sequential samples", t, func() {
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

		stage := NewLogReturn(config)
		var lastArtifact *datura.Artifact

		for _, sample := range []float64{100, 101, 102} {
			artifact := datura.Acquire("log-return-test", datura.APPJSON)
			artifact.Merge("sample", sample)

			err := transport.NewFlipFlop(artifact, stage)

			So(err, ShouldBeNil)

			if lastArtifact != nil {
				lastArtifact.Release()
			}

			lastArtifact = artifact
		}

		defer lastArtifact.Release()

		Convey("It should publish a positive log return on the wire sample", func() {
			So(datura.Peek[string](lastArtifact, "root"), ShouldEqual, "sample")
			So(datura.Peek[float64](lastArtifact, "sample"), ShouldBeGreaterThan, 0)
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
		}, "inputs")

	stage := NewLogReturn(config)

	b.ReportAllocs()

	for b.Loop() {
		artifact := datura.Acquire("log-return-bench-test", datura.APPJSON)
		artifact.Merge("sample", 103.0)
		_ = transport.NewFlipFlop(artifact, stage)
		artifact.Release()
	}
}
