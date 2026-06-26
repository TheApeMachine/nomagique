package adaptive

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

var compressionInput = ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10)

func compressionStageConfig() *datura.Artifact {
	return datura.Acquire("compression-config", datura.APPJSON).
		Poke(map[string]any{
			"input":     "sample",
			"outputKey": "value",
			"seriesKey": "compression",
		}, "compression")
}

func TestCompressionZeroOutput(t *testing.T) {
	Convey("Given a legitimate zero spread in output", t, func() {
		config := datura.Acquire("compression-config-zero", datura.APPJSON).
			Poke(map[string]any{
				"input":     "spread",
				"outputKey": "compression",
				"seriesKey": "spread",
			}, "compression")
		compression := NewCompression(config)
		warmup := datura.Acquire("compression-warmup-frame", datura.APPJSON)
		warmup.Poke("output", "root")
		warmup.Poke([]string{"spread"}, "inputs")
		warmup.Merge("output", map[string]any{"spread": 2.0})

		err := nomagique.RoundTripArtifact(warmup, compression)

		So(err, ShouldBeNil)

		frame := datura.Acquire("compression-zero-frame", datura.APPJSON)
		frame.Poke("output", "root")
		frame.Poke([]string{"spread"}, "inputs")
		frame.Merge("output", map[string]any{"spread": 0.0})

		err = nomagique.RoundTripArtifact(frame, compression)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](frame, "output", "compression"), ShouldEqual, 1)
	})
}

func TestCompressionRead(t *testing.T) {
	Convey("Given a Compression on the first sample", t, func() {
		compression := NewCompression(compressionStageConfig())
		_, _ = nomagique.WriteArtifact(compression, compressionInput)

		frame := make([]byte, 65536)
		readCount, err := compression.Read(frame)

		So(err, ShouldEqual, io.EOF)
		So(readCount, ShouldBeGreaterThan, 0)

		outbound := datura.Acquire("test-out", datura.APPJSON)
		_, err = outbound.Unpack(frame[:readCount])
		So(err, ShouldBeNil)
		So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 0)
	})

	Convey("Given a warmed Compression", t, func() {
		compression := NewCompression(compressionStageConfig())
		_, _ = nomagique.WriteArtifact(compression, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))
		_, _ = compression.Read(make([]byte, 65536))
		_, _ = nomagique.WriteArtifact(compression, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 8))

		frame := make([]byte, 65536)
		readCount, err := compression.Read(frame)

		So(err, ShouldEqual, io.EOF)
		So(readCount, ShouldBeGreaterThan, 0)

		outbound := datura.Acquire("test-out", datura.APPJSON)
		_, err = outbound.Unpack(frame[:readCount])
		So(err, ShouldBeNil)
		So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 0.2)
	})

	Convey("Given missing compression config", t, func() {
		compression := NewCompression(datura.Acquire("compression-config-missing", datura.APPJSON))
		_, _ = nomagique.WriteArtifact(compression, compressionInput)

		_, err := compression.Read(make([]byte, 65536))

		So(err, ShouldNotBeNil)
	})
}

func TestCompressionWrite(t *testing.T) {
	Convey("Given a Compression", t, func() {
		compression := NewCompression(compressionStageConfig())

		Convey("When Write is called", func() {
			_, err := nomagique.WriteArtifact(compression, compressionInput)
			So(err, ShouldBeNil)
		})
	})
}
