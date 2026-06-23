package adaptive

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func logReturnConfig() *datura.Artifact {
	return datura.Acquire("log-return-config", datura.APPJSON).
		Poke([]string{"rvol", "precursor"}, "order").
		Poke(1.0, "stageIndex").
		Poke(map[string]any{
			"input":      "last",
			"returnLag":  1.0,
			"longWindow": 5.0,
			"outputKey":  "precursor",
		}, "precursor")
}

func TestLogReturnRead(t *testing.T) {
	Convey("Given a log-return stage fed sequential samples", t, func() {
		config := logReturnConfig()
		stage := NewLogReturn(config)
		var lastArtifact *datura.Artifact

		for _, sample := range []float64{100, 101, 102} {
			artifact := datura.Acquire("log-return-test", datura.APPJSON)
			artifact.Merge("last", sample)

			err := transport.NewFlipFlop(artifact, stage)

			if sample == 100 {
				So(err, ShouldNotBeNil)
			}

			if lastArtifact != nil {
				lastArtifact.Release()
			}

			lastArtifact = artifact
		}

		defer lastArtifact.Release()

		Convey("It should publish a positive log return under output", func() {
			So(datura.Peek[string](lastArtifact, "root"), ShouldEqual, "output")
			So(datura.Peek[float64](lastArtifact, "output", "precursor"), ShouldBeGreaterThan, 0)
			So(
				datura.Peek[float64](lastArtifact, "output", "precursor"),
				ShouldAlmostEqual,
				math.Log(102.0/101.0),
				0.0001,
			)
		})
	})
}

func BenchmarkLogReturnRead(b *testing.B) {
	config := logReturnConfig()
	stage := NewLogReturn(config)
	artifact := datura.Acquire("log-return-bench-test", datura.APPJSON)

	for _, sample := range []float64{100, 101, 102} {
		artifact.Merge("sample", sample)
		_ = transport.NewFlipFlop(artifact, stage)
	}

	b.ReportAllocs()

	for b.Loop() {
		artifact.Merge("sample", 103.0)
		_, _ = stage.Read(make([]byte, 65536))
	}
}
