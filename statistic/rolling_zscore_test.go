package statistic

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"gonum.org/v1/gonum/stat"
)

func rollingZScoreConfig() *datura.Artifact {
	return datura.Acquire("rolling-zscore-config", datura.APPJSON).
		Poke("sample", "input").
		Poke("value", "outputKey").
		Poke("test", "seriesKey").
		Poke("test", "seriesKey")
}

func rollingZScoreFrame(sample float64, timestamp int64) *datura.Artifact {
	artifact := ScalarWire(datura.Acquire("rolling-zscore-test", datura.APPJSON), "sample", sample)
	artifact.SetTimestamp(timestamp)

	return artifact
}

func TestRollingZScoreRead(t *testing.T) {
	Convey("Given a rolling z-score stage fed sequential samples", t, func() {
		config := rollingZScoreConfig()
		stage := NewRollingZScore(config)
		samples := []float64{-0.01, 0.0, 0.01, 0.02, 0.03}
		var lastArtifact *datura.Artifact
		timestamp := time.Unix(0, 1).UnixNano()

		for index, sample := range samples {
			artifact := rollingZScoreFrame(sample, timestamp)
			timestamp += int64(time.Second)

			err := transport.NewFlipFlop(artifact, stage)

			So(err, ShouldBeNil)

			if index == 0 {
				So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)
			}

			if index >= 2 {
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
		config := rollingZScoreConfig()
		stage := NewRollingZScore(config)
		var lastArtifact *datura.Artifact
		timestamp := time.Unix(0, 1).UnixNano()

		for range 6 {
			artifact := rollingZScoreFrame(0.0, timestamp)
			timestamp += int64(time.Second)

			err := transport.NewFlipFlop(artifact, stage)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldAlmostEqual, 0, 1e-9)

			if lastArtifact != nil {
				lastArtifact.Release()
			}

			lastArtifact = artifact
		}

		breakout := rollingZScoreFrame(0.05, timestamp)
		timestamp += int64(time.Second)

		err := transport.NewFlipFlop(breakout, stage)
		So(err, ShouldBeNil)
		So(datura.Peek[float64](breakout, "output", "value"), ShouldAlmostEqual, 1, 1e-9)

		followThrough := rollingZScoreFrame(0.06, timestamp)

		err = transport.NewFlipFlop(followThrough, stage)
		breakout.Release()

		if lastArtifact != nil {
			lastArtifact.Release()
		}

		defer followThrough.Release()

		Convey("It should advance retained samples through zero-variance priors", func() {
			sampleCount := 0

			for _, samples := range stage.samples {
				sampleCount += len(samples)
			}

			So(sampleCount, ShouldEqual, 8)
			So(err, ShouldBeNil)
			So(datura.Peek[float64](followThrough, "output", "value"), ShouldNotEqual, 0)
		})
	})
}

func BenchmarkRollingZScoreRead(b *testing.B) {
	config := rollingZScoreConfig()
	stage := NewRollingZScore(config)

	b.ReportAllocs()

	for b.Loop() {
		artifact := rollingZScoreFrame(0.03, time.Unix(0, 1).UnixNano())
		_ = transport.NewFlipFlop(artifact, stage)
		artifact.Release()
	}
}
