package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestRollingZScoreRead(t *testing.T) {
	Convey("Given a rolling z-score stage fed sequential samples", t, func() {
		config := datura.Acquire("rolling-zscore-config", datura.APPJSON)
		stage := NewRollingZScore(config)
		var lastArtifact *datura.Artifact

		for _, sample := range []float64{-0.01, 0.0, 0.01, 0.02, 0.03} {
			artifact := datura.Acquire("rolling-zscore-test", datura.APPJSON)
			artifact.Merge("sample", sample)

			err := transport.NewFlipFlop(artifact, stage)

			So(err, ShouldBeNil)

			if lastArtifact != nil {
				lastArtifact.Release()
			}

			lastArtifact = artifact
		}

		defer lastArtifact.Release()

		Convey("It should normalize the current sample into output", func() {
			So(datura.Peek[float64](lastArtifact, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkRollingZScoreRead(b *testing.B) {
	config := datura.Acquire("rolling-zscore-bench", datura.APPJSON)
	stage := NewRollingZScore(config)

	b.ReportAllocs()

	for b.Loop() {
		artifact := datura.Acquire("rolling-zscore-bench-test", datura.APPJSON)
		artifact.Merge("sample", 0.03)
		_ = transport.NewFlipFlop(artifact, stage)
		artifact.Release()
	}
}
