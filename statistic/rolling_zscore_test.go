package statistic

import (
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"gonum.org/v1/gonum/stat"
)

func rollingZScoreConfig() *datura.Artifact {
	return datura.Acquire("rolling-zscore-config", datura.APPJSON).
		Poke("rolling", "stage").
		Poke("sample", "rolling", "input").
		Poke("value", "rolling", "outputKey").
		Poke("rolling", "rolling", "seriesKey")
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

			if index == 0 {
				So(err, ShouldNotBeNil)

				artifact.Release()

				continue
			}

			So(err, ShouldBeNil)

			prior := samples[:index]

			if len(prior) == 1 {
				continue
			}

			meanSample := stat.Mean(prior, nil)
			stdSample := stat.StdDev(prior, nil)

			if stdSample <= 0 || math.IsNaN(stdSample) {
				continue
			}

			expected := (sample - meanSample) / stdSample

			So(
				datura.Peek[float64](artifact, "output", "value"),
				ShouldAlmostEqual,
				expected,
				1e-9,
			)

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

		for index := range 6 {
			artifact := rollingZScoreFrame(0.0, timestamp)
			timestamp += int64(time.Second)

			err := transport.NewFlipFlop(artifact, stage)

			if index == 0 {
				So(err, ShouldNotBeNil)

				artifact.Release()

				continue
			}

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldAlmostEqual, 0, 1e-9)

			if lastArtifact != nil {
				lastArtifact.Release()
			}

			lastArtifact = artifact
		}

		artifact := rollingZScoreFrame(0.1, timestamp)

		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](artifact, "output", "value"), ShouldNotEqual, 0)

		if lastArtifact != nil {
			lastArtifact.Release()
		}

		lastArtifact = artifact
		defer lastArtifact.Release()
	})
}

func BenchmarkRollingZScoreRead(b *testing.B) {
	config := rollingZScoreConfig()
	stage := NewRollingZScore(config)
	timestamp := time.Unix(0, 1).UnixNano()
	artifact := rollingZScoreFrame(0.01, timestamp)

	b.ReportAllocs()

	for b.Loop() {
		timestamp += int64(time.Second)
		artifact.SetTimestamp(timestamp)
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
