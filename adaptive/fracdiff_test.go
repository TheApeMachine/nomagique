package adaptive

import (
	"io"
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var fracDiffInput = datura.Acquire("test", datura.APPJSON).Poke(10, "sample")

func TestFracDiffRead(t *testing.T) {
	Convey("Given a FracDiff on the first sample", t, func() {
		fractional := NewFracDiff(datura.Acquire("fracdiff-config", datura.APPJSON))
		_, _ = io.Copy(fractional, fracDiffInput)

		frame := make([]byte, 65536)
		_, err := fractional.Read(frame)

		So(err, ShouldNotBeNil)
		So(datura.KeyPresent(fractional.artifact, "output", "count"), ShouldBeTrue)
	})

	Convey("Given a second FracDiff sample after bootstrap", t, func() {
		fractional := NewFracDiff(datura.Acquire("fracdiff-config", datura.APPJSON))
		_, _ = io.Copy(fractional, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))
		_, _ = fractional.Read(make([]byte, 65536))
		_, _ = io.Copy(fractional, datura.Acquire("test", datura.APPJSON).Poke(11, "sample"))

		frame := make([]byte, 65536)
		readCount, err := fractional.Read(frame)

		So(err, ShouldEqual, io.EOF)
		So(readCount, ShouldBeGreaterThan, 0)
		So(datura.Peek[float64](fractional.artifact, "output", "value"), ShouldNotEqual, 0)
	})

	Convey("Given a repeated span after bootstrap", t, func() {
		fractional := NewFracDiff(datura.Acquire("fracdiff-config", datura.APPJSON))
		_, _ = io.Copy(fractional, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))
		_, _ = fractional.Read(make([]byte, 65536))
		_, _ = io.Copy(fractional, datura.Acquire("test", datura.APPJSON).Poke(10, "sample"))

		_, err := fractional.Read(make([]byte, 65536))

		So(err, ShouldNotBeNil)
	})

	Convey("Given a non-finite sample", t, func() {
		fractional := NewFracDiff(datura.Acquire("fracdiff-config", datura.APPJSON))
		invalid := datura.Acquire("test", datura.APPJSON).Poke(math.NaN(), "sample")
		_, _ = io.Copy(fractional, invalid)

		_, err := fractional.Read(make([]byte, 65536))

		So(err, ShouldNotBeNil)
	})
}

func TestFracDiffWrite(t *testing.T) {
	Convey("Given a FracDiff", t, func() {
		fractional := NewFracDiff(datura.Acquire("fracdiff-config", datura.APPJSON))

		Convey("When Write is called", func() {
			_, err := io.Copy(fractional, fracDiffInput)
			So(err, ShouldBeNil)
		})
	})
}
