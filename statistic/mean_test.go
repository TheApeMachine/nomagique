package statistic

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func TestMeanRead(t *testing.T) {
	Convey("Given a Mean", t, func() {
		config := datura.Acquire("mean-config", datura.APPJSON).Poke("sample", "input")
		mean := NewMean(config)
		input := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 1)
		_, err := nomagique.WriteArtifact(mean, input)

		So(err, ShouldBeNil)

		Convey("When Read is called", func() {
			frame := make([]byte, 65536)
			readCount, err := mean.Read(frame)
			So(err, ShouldEqual, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](mean.artifact, "output", "value"), ShouldEqual, 1)
		})
	})
}

func TestMeanWrite(t *testing.T) {
	Convey("Given a Mean", t, func() {
		config := datura.Acquire("mean-config-write", datura.APPJSON).Poke("sample", "input")
		mean := NewMean(config)

		Convey("When Write is called", func() {
			input := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 1)
			_, err := nomagique.WriteArtifact(mean, input)
			So(err, ShouldBeNil)
		})
	})
}

func TestMeanSeries(t *testing.T) {
	Convey("Given a sample series", t, func() {
		config := datura.Acquire("mean-config-series", datura.APPJSON).Poke("sample", "input")
		mean := NewMean(config)
		var lastArtifact *datura.Artifact

		for _, sample := range []float64{1, 2, 3, 4} {
			artifact := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", sample)
			err := nomagique.RoundTripArtifact(artifact, mean)

			So(err, ShouldBeNil)

			if lastArtifact != nil {
				lastArtifact.Release()
			}

			lastArtifact = artifact
		}

		defer lastArtifact.Release()

		got := datura.Peek[float64](lastArtifact, "output", "value")

		Convey("It should return the running mean", func() {
			So(got, ShouldEqual, 2.5)
		})
	})
}

func BenchmarkMeanRead(b *testing.B) {
	config := datura.Acquire("mean-config-bench", datura.APPJSON).Poke("sample", "input")
	mean := NewMean(config)
	input := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 1)
	_, _ = nomagique.WriteArtifact(mean, input)
	frame := make([]byte, 65536)

	b.ReportAllocs()

	for b.Loop() {
		_, _ = mean.Read(frame)
	}
}
