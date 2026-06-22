package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"gonum.org/v1/gonum/stat"
)

func TestRollingZScoreRead(t *testing.T) {
	Convey("Given a rolling z-score stage fed sequential samples", t, func() {
		config := datura.Acquire("rolling-zscore-config", datura.APPJSON)
		stage := NewRollingZScore(config)
		samples := []float64{-0.01, 0.0, 0.01, 0.02, 0.03}
		var lastArtifact *datura.Artifact

		for index, sample := range samples {
			artifact := datura.Acquire("rolling-zscore-test", datura.APPJSON)
			artifact.Merge("sample", sample)

			err := transport.NewFlipFlop(artifact, stage)

			if index < 2 {
				So(err, ShouldNotBeNil)
			}

			if index >= 2 {
				So(err, ShouldBeNil)

				prior := samples[:index]
				meanSample := stat.Mean(prior, nil)
				stdSample := stat.StdDev(prior, nil)
				expected := (sample - meanSample) / stdSample

				So(
					datura.Peek[float64](artifact, "output", "value"),
					ShouldAlmostEqual,
					expected,
					1e-9,
				)
			}

			if lastArtifact != nil {
				lastArtifact.Release()
			}

			lastArtifact = artifact
		}

		defer lastArtifact.Release()

		Convey("It should leave the current sample out of the prior statistics", func() {
			prior := samples[:len(samples)-1]
			sample := samples[len(samples)-1]
			meanSample := stat.Mean(prior, nil)
			stdSample := stat.StdDev(prior, nil)
			expected := (sample - meanSample) / stdSample

			So(
				datura.Peek[float64](lastArtifact, "output", "value"),
				ShouldAlmostEqual,
				expected,
				1e-9,
			)
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
