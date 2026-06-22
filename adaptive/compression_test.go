package adaptive

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

var compressionInput = datura.Acquire("test", datura.APPJSON).Poke(10, "sample")

var compressionConfig = datura.Acquire("compression-config", datura.APPJSON)

func TestCompressionZeroOutput(t *testing.T) {
	Convey("Given a legitimate zero spread in output", t, func() {
		config := datura.Acquire("compression-config-zero", datura.APPJSON).
			Poke(map[string]any{
				"input":     "spread",
				"outputKey": "compression",
			}, "compression")
		compression := NewCompression(config)
		frame := datura.Acquire("compression-zero-frame", datura.APPJSON)
		frame.Merge("root", "output")
		frame.Merge("output", map[string]any{"spread": 0.0})

		err := transport.NewFlipFlop(frame, compression)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](frame, "output", "compression"), ShouldEqual, 0)
	})
}

func TestCompressionRead(t *testing.T) {
	Convey("Given a Compression", t, func() {
		compression := NewCompression(compressionConfig)
		io.Copy(compression, compressionInput)

		Convey("When Read is called", func() {
			frame := make([]byte, 65536)
			readCount, err := compression.Read(frame)
			So(err, ShouldEqual, io.EOF)

			outbound := datura.Acquire("test-out", datura.APPJSON)
			_, err = outbound.Write(frame[:readCount])
			So(err, ShouldBeNil)

			rootKey := datura.Peek[string](outbound, "root")

			So(rootKey, ShouldEqual, "output")
			So(datura.Peek[float64](outbound, rootKey, "value"), ShouldEqual, 0)
		})
	})
}

func TestCompressionWrite(t *testing.T) {
	Convey("Given a Compression", t, func() {
		compression := NewCompression(compressionConfig)

		Convey("When Write is called", func() {
			_, err := io.Copy(compression, compressionInput)
			So(err, ShouldBeNil)
		})
	})
}
