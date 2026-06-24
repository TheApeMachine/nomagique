package adaptive

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var accumulatorInput = ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 1)

func TestAccumulatorRead(t *testing.T) {
	Convey("Given an Accumulator", t, func() {
		accumulator := NewAccumulator(datura.Acquire("accumulator-config", datura.APPJSON).Poke("sample", "input"))
		io.Copy(accumulator, accumulatorInput)

		Convey("When Read is called", func() {
			frame := make([]byte, 65536)
			readCount, err := accumulator.Read(frame)
			So(err, ShouldEqual, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)

			outbound := datura.Acquire("accumulator-outbound", datura.APPJSON)
			_, _ = outbound.Write(frame[:readCount])
			So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 1)
		})
	})
}

func TestAccumulatorWrite(t *testing.T) {
	Convey("Given an Accumulator", t, func() {
		accumulator := NewAccumulator(datura.Acquire("accumulator-config", datura.APPJSON).Poke("sample", "input"))

		Convey("When Write is called", func() {
			_, err := io.Copy(accumulator, accumulatorInput)
			So(err, ShouldBeNil)
		})
	})
}
