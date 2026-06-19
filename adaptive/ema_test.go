package adaptive

import (
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
			_, err := ema.Read([]byte{1, 2, 3})
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
	})
}
