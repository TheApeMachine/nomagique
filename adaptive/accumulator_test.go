package adaptive

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var accumulatorInput = datura.Acquire("test", datura.APPJSON).Poke(1, "sample")

func TestAccumulatorRead(t *testing.T) {
	Convey("Given an Accumulator", t, func() {
		accumulator := NewAccumulator(datura.Acquire("accumulator-config", datura.APPJSON))
		io.Copy(accumulator, accumulatorInput)

		Convey("When Read is called", func() {
			frame := make([]byte, 65536)
			readCount, err := accumulator.Read(frame)
			So(err, ShouldEqual, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](accumulator.artifact, "output", "value"), ShouldEqual, 1)
		})
	})
}

func TestAccumulatorWrite(t *testing.T) {
	Convey("Given an Accumulator", t, func() {
		accumulator := NewAccumulator(datura.Acquire("accumulator-config", datura.APPJSON))

		Convey("When Write is called", func() {
			_, err := io.Copy(accumulator, accumulatorInput)
			So(err, ShouldBeNil)
		})
	})
}
