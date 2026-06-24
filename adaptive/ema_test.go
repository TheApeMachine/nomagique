package adaptive

import (
	"io"
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func emaConfigArtifact() *datura.Artifact {
	return datura.Acquire("ema-config", datura.APPJSON).
		Poke("sample", "input").
		Poke(2, "period").
		Poke(2, "smoothing")
}

func TestEMARead(t *testing.T) {
	Convey("Given an EMA on the first sample", t, func() {
		ema := NewEMA(emaConfigArtifact())
		_, _ = io.Copy(ema, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 1))

		frame := make([]byte, 65536)
		_, err := ema.Read(frame)

		So(err, ShouldNotBeNil)
	})

	Convey("Given a second EMA sample after bootstrap", t, func() {
		ema := NewEMA(emaConfigArtifact())
		_, _ = io.Copy(ema, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 1))
		_, _ = ema.Read(make([]byte, 65536))
		artifact := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 2)
		_, _ = io.Copy(ema, artifact)

		frame := make([]byte, 65536)
		readCount, err := ema.Read(frame)

		So(err, ShouldEqual, io.EOF)
		So(readCount, ShouldBeGreaterThan, 0)

		err = transport.NewFlipFlop(artifact, ema)

		So(err, ShouldBeIn, nil, io.EOF)
		So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
	})

	Convey("Given a repeated sample after bootstrap", t, func() {
		ema := NewEMA(emaConfigArtifact())
		_, _ = io.Copy(ema, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 1))
		_, _ = ema.Read(make([]byte, 65536))
		artifact := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 1)
		_, _ = io.Copy(ema, artifact)

		frame := make([]byte, 65536)
		readCount, err := ema.Read(frame)

		So(err, ShouldEqual, io.EOF)
		So(readCount, ShouldBeGreaterThan, 0)
	})

	Convey("Given a non-finite sample", t, func() {
		ema := NewEMA(emaConfigArtifact())
		invalid := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", math.NaN())
		_, _ = io.Copy(ema, invalid)

		_, err := ema.Read(make([]byte, 65536))

		So(err, ShouldNotBeNil)
	})
}

func TestEMAWrite(t *testing.T) {
	Convey("Given an EMA", t, func() {
		ema := NewEMA(emaConfigArtifact())

		Convey("When Write is called", func() {
			_, err := io.Copy(ema, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 1))
			So(err, ShouldBeIn, nil, io.EOF)
		})
	})
}

func TestEMAFlipFlop(t *testing.T) {
	Convey("Given an EMA fed through FlipFlop", t, func() {
		ema := NewEMA(emaConfigArtifact())
		bootstrap := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 1)

		So(transport.NewFlipFlop(bootstrap, ema), ShouldBeIn, nil, io.EOF)
		bootstrap.Release()

		artifact := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 2)

		err := transport.NewFlipFlop(artifact, ema)

		So(err, ShouldBeIn, nil, io.EOF)
		So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)

		rootKey := datura.Peek[string](artifact, "root")

		Convey("It should publish root and inputs for downstream navigation", func() {
			So(rootKey, ShouldEqual, "output")
			So(datura.Peek[[]string](artifact, "inputs"), ShouldResemble, []string{"value"})
			So(datura.Peek[float64](artifact, rootKey, "value"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkEMARead(b *testing.B) {
	ema := NewEMA(emaConfigArtifact())
	buffer := make([]byte, 65536)

	b.ReportAllocs()

	for b.Loop() {
		inbound := ScalarWire(datura.Acquire("bench-inbound", datura.APPJSON), "sample", 1)

		if _, err := io.Copy(ema, inbound); err != nil {
			b.Fatal(err)
		}

		if _, err := ema.Read(buffer); err != nil && err != io.EOF {
			b.Fatal(err)
		}

		inbound.Release()
	}
}
