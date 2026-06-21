package adaptive

import (
	"bytes"
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

var emaInput = datura.Acquire("test", datura.APPJSON).Poke(1, "sample")

var emaConfig = datura.Acquire("ema-config", datura.APPJSON)

func TestEMARead(t *testing.T) {
	Convey("Given an EMA", t, func() {
		ema := NewEMA(emaConfig)
		io.Copy(ema, emaInput)

		Convey("When Read is called", func() {
			buffer := bytes.NewBuffer(nil)
			_, err := io.Copy(buffer, ema)
			So(err, ShouldBeNil)
			So(datura.Peek[float64](ema.artifact, "output", "value"), ShouldEqual, 1)
		})
	})
}

func TestEMAWrite(t *testing.T) {
	Convey("Given an EMA", t, func() {
		ema := NewEMA(emaConfig)

		Convey("When Write is called", func() {
			_, err := io.Copy(ema, emaInput)
			So(err, ShouldBeNil)
		})
	})
}

func TestEMAFlipFlop(t *testing.T) {
	Convey("Given an EMA fed through FlipFlop", t, func() {
		ema := NewEMA(emaConfig)
		artifact := datura.Acquire("test", datura.APPJSON).Poke(2, "sample")

		err := transport.NewFlipFlop(artifact, ema)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 2)

		rootKey := datura.Peek[string](artifact, "root")

		Convey("It should publish root and inputs for downstream navigation", func() {
			So(rootKey, ShouldEqual, "output")
			So(datura.Peek[[]string](artifact, "inputs"), ShouldResemble, []string{"value"})
			So(datura.Peek[float64](artifact, rootKey, "value"), ShouldEqual, 2)
		})
	})
}

func BenchmarkEMARead(b *testing.B) {
	ema := NewEMA(datura.Acquire("ema-config", datura.APPJSON))
	buffer := make([]byte, 65536)

	b.ReportAllocs()

	for range b.N {
		inbound := datura.Acquire("bench-inbound", datura.APPJSON).Poke(1, "sample")

		if _, err := io.Copy(ema, inbound); err != nil {
			b.Fatal(err)
		}

		if _, err := ema.Read(buffer); err != nil && err != io.EOF {
			b.Fatal(err)
		}

		inbound.Release()
	}
}
