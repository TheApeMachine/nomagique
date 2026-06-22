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

	Convey("Given a flat prior that later gains variance", t, func() {
		config := datura.Acquire("rolling-zscore-flat-config", datura.APPJSON)
		stage := NewRollingZScore(config)
		var lastArtifact *datura.Artifact

		for index := range 6 {
			artifact := datura.Acquire("rolling-zscore-flat-test", datura.APPJSON)
			artifact.Merge("sample", 0.0)

			err := transport.NewFlipFlop(artifact, stage)

			if index < 2 {
				So(err, ShouldNotBeNil)
			}

			if index >= 2 {
				So(err, ShouldBeNil)
				So(datura.Peek[float64](artifact, "output", "value"), ShouldAlmostEqual, 0, 1e-9)
			}

			if lastArtifact != nil {
				lastArtifact.Release()
			}

			lastArtifact = artifact
		}

		breakout := datura.Acquire("rolling-zscore-breakout-test", datura.APPJSON)
		breakout.Merge("sample", 0.05)

		err := transport.NewFlipFlop(breakout, stage)
		So(err, ShouldBeNil)
		So(datura.Peek[float64](breakout, "output", "value"), ShouldAlmostEqual, 1, 1e-9)

		followThrough := datura.Acquire("rolling-zscore-follow-test", datura.APPJSON)
		followThrough.Merge("sample", 0.06)

		err = transport.NewFlipFlop(followThrough, stage)
		breakout.Release()

		if lastArtifact != nil {
			lastArtifact.Release()
		}

		defer followThrough.Release()

		Convey("It should advance retained samples through zero-variance priors", func() {
			So(len(stage.samples), ShouldEqual, 8)
			So(err, ShouldBeNil)
			So(datura.Peek[float64](followThrough, "output", "value"), ShouldNotEqual, 0)
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
