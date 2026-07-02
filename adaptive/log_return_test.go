package adaptive

import (
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func logReturnConfig() *datura.Artifact {
	return datura.Acquire("log-return-config", datura.APPJSON).
		Poke("precursor", "stage").
		Poke(map[string]any{
			"input":      "last",
			"returnLag":  1.0,
			"longWindow": 5.0,
			"outputKey":  "precursor",
		}, "precursor")
}

func logReturnFrame(last float64, timestamp int64) *datura.Artifact {
	artifact := datura.Acquire("log-return-test", datura.APPJSON)
	artifact.Poke("features", "root")
	artifact.Poke([]string{"volume", "last"}, "inputs")
	artifact.Merge("features", []float64{100, last})
	artifact.SetTimestamp(timestamp)

	return artifact
}

func TestLogReturnRead(t *testing.T) {
	Convey("Given a log-return stage fed sequential samples", t, func() {
		config := logReturnConfig()
		stage := NewLogReturn(config)
		var lastArtifact *datura.Artifact
		timestamp := time.Unix(0, 1).UnixNano()

		for index, sample := range []float64{100, 101, 102} {
			artifact := logReturnFrame(sample, timestamp)
			timestamp += int64(time.Second)

			err := nomagique.RoundTripArtifact(artifact, stage)

			if index == 0 {
				So(err, ShouldBeNil)
				So(datura.Peek[float64](artifact, "output", "precursor"), ShouldEqual, 0)

				artifact.Release()

				continue
			}

			So(err, ShouldBeNil)

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

func TestLogReturnReadLongReplayDoesNotGrowTraversal(t *testing.T) {
	Convey("Given a long-lived log-return stage", t, func() {
		config := logReturnConfig()
		stage := NewLogReturn(config)
		defer stage.Close()

		var lastArtifact *datura.Artifact
		timestamp := time.Unix(0, 1).UnixNano()

		for index := range 2500 {
			artifact := logReturnFrame(100.0+float64(index)*0.01, timestamp)
			timestamp += int64(time.Millisecond)

			err := nomagique.RoundTripArtifact(artifact, stage)

			if index == 0 {
				So(err, ShouldBeNil)
				So(datura.Peek[float64](artifact, "output", "precursor"), ShouldEqual, 0)

				artifact.Release()

				continue
			}

			So(err, ShouldBeNil)

			if lastArtifact != nil {
				lastArtifact.Release()
			}

			lastArtifact = artifact
		}

		defer lastArtifact.Release()

		Convey("It should keep emitting output", func() {
			So(datura.Peek[float64](lastArtifact, "output", "precursor"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkLogReturnRead(b *testing.B) {
	config := logReturnConfig()
	stage := NewLogReturn(config)
	timestamp := time.Unix(0, 1).UnixNano()

	for _, sample := range []float64{100, 101, 102} {
		artifact := logReturnFrame(sample, timestamp)
		timestamp += int64(time.Second)
		_ = nomagique.RoundTripArtifact(artifact, stage)
		artifact.Release()
	}

	b.ReportAllocs()

	for b.Loop() {
		artifact := logReturnFrame(103.0, timestamp)
		timestamp += int64(time.Second)
		_ = nomagique.RoundTripArtifact(artifact, stage)
		artifact.Release()
	}
}
