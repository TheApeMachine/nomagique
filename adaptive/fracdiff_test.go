package adaptive

import (
	"io"
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

var fracDiffInput = ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10)

func TestFracDiffRead(t *testing.T) {
	Convey("Given a FracDiff on the first sample", t, func() {
		fractional := NewFracDiff(datura.Acquire("fracdiff-config", datura.APPJSON).Poke("sample", "input"))
		_, _ = nomagique.WriteArtifact(fractional, fracDiffInput)

		frame := make([]byte, 65536)
		readCount, err := fractional.Read(frame)

		So(err, ShouldEqual, io.EOF)
		So(readCount, ShouldBeGreaterThan, 0)

		outbound := datura.Acquire("fracdiff-outbound", datura.APPJSON)
		_, _ = outbound.Unpack(frame[:readCount])
		So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 0)
		So(datura.Peek[bool](outbound, "output", "ready"), ShouldBeFalse)
	})

	Convey("Given a second FracDiff sample after bootstrap", t, func() {
		fractional := NewFracDiff(datura.Acquire("fracdiff-config", datura.APPJSON).Poke("sample", "input"))
		_, _ = nomagique.WriteArtifact(fractional, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))
		_, _ = fractional.Read(make([]byte, 65536))
		artifact := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 11)
		_, _ = nomagique.WriteArtifact(fractional, artifact)

		frame := make([]byte, 65536)
		readCount, err := fractional.Read(frame)

		So(err, ShouldEqual, io.EOF)
		So(readCount, ShouldBeGreaterThan, 0)

		err = nomagique.RoundTripArtifact(artifact, fractional)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](artifact, "output", "value"), ShouldNotEqual, 0)
		So(datura.Peek[bool](artifact, "output", "ready"), ShouldBeTrue)
	})

	Convey("Given a repeated span after bootstrap", t, func() {
		fractional := NewFracDiff(datura.Acquire("fracdiff-config", datura.APPJSON).Poke("sample", "input"))
		_, _ = nomagique.WriteArtifact(fractional, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))
		_, _ = fractional.Read(make([]byte, 65536))
		_, _ = nomagique.WriteArtifact(fractional, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))

		frame := make([]byte, 65536)
		readCount, err := fractional.Read(frame)

		So(err, ShouldEqual, io.EOF)
		So(readCount, ShouldBeGreaterThan, 0)

		outbound := datura.Acquire("fracdiff-outbound", datura.APPJSON)
		_, _ = outbound.Unpack(frame[:readCount])
		So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 0)
		So(datura.Peek[bool](outbound, "output", "ready"), ShouldBeFalse)
	})

	Convey("Given a non-finite sample", t, func() {
		fractional := NewFracDiff(datura.Acquire("fracdiff-config", datura.APPJSON).Poke("sample", "input"))
		invalid := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", math.NaN())
		_, _ = nomagique.WriteArtifact(fractional, invalid)

		_, err := fractional.Read(make([]byte, 65536))

		So(err, ShouldNotBeNil)
	})
}

func TestFracDiffWrite(t *testing.T) {
	Convey("Given a FracDiff", t, func() {
		fractional := NewFracDiff(datura.Acquire("fracdiff-config", datura.APPJSON).Poke("sample", "input"))

		Convey("When Write is called", func() {
			_, err := nomagique.WriteArtifact(fractional, fracDiffInput)
			So(err, ShouldBeNil)
		})
	})
}
