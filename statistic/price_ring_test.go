package statistic

import (
	"io"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func precursorConfig() *datura.Artifact {
	return datura.Acquire("precursor-config", datura.APPJSON).
		Poke("last", "input").
		Poke(1.0, "returnLag").
		Poke(5.0, "longWindow")
}

func precursorState(last float64) *datura.Artifact {
	artifact := datura.Acquire("precursor-state", datura.APPJSON)
	artifact.Poke("features", "root")
	artifact.Poke([]string{"volume", "last"}, "inputs")
	artifact.Merge("features", []float64{100, last})
	artifact.SetTimestamp(time.Unix(0, 1).UnixNano())

	return artifact
}

func TestPriceRingRead(t *testing.T) {
	Convey("Given a price ring stage", t, func() {
		config := precursorConfig()
		stage := NewPriceRing(config)
		var lastArtifact *datura.Artifact

		Convey("It should return EOF before a frame is written", func() {
			payload := make([]byte, 1024)
			read, err := stage.Read(payload)

			So(read, ShouldEqual, 0)
			So(err, ShouldEqual, io.EOF)
		})

		Convey("It should clear an empty write without parsing stale state", func() {
			written, err := stage.Write(nil)
			So(written, ShouldEqual, 0)
			So(err, ShouldBeNil)

			payload := make([]byte, 1024)
			read, err := stage.Read(payload)

			So(read, ShouldEqual, 0)
			So(err, ShouldEqual, io.EOF)
		})

		for _, last := range []float64{100, 101, 102} {
			artifact := precursorState(last)
			artifact.SetTimestamp(artifact.Timestamp() + int64(time.Second))
			err := nomagique.RoundTripArtifact(artifact, stage)
			So(err, ShouldBeNil)
			lastArtifact = artifact
		}

		Convey("It should publish the current sample on the outbound wire", func() {
			So(datura.Peek[string](lastArtifact, "root"), ShouldEqual, "output")
			So(datura.Peek[float64](lastArtifact, "output", "last"), ShouldEqual, 102)
		})
	})
}

func BenchmarkPriceRingRead(b *testing.B) {
	config := precursorConfig()
	stage := NewPriceRing(config)

	b.ReportAllocs()

	for b.Loop() {
		artifact := datura.Acquire("precursor-state", datura.APPJSON)
		artifact.Poke("features", "root")
		artifact.Poke([]string{"volume", "last"}, "inputs")
		artifact.WithPayload(datura.Map[any]{
			"features": []float64{100, 101},
		}.Marshal())
		_ = nomagique.RoundTripArtifact(artifact, stage)
		artifact.Release()
	}
}
