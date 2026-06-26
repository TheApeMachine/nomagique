package adaptive

import (
	"io"
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

var momentumInput = ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10)

func TestMomentumRead(t *testing.T) {
	Convey("Given a Momentum on the first sample", t, func() {
		momentum := NewMomentum(datura.Acquire("momentum-config", datura.APPJSON).Poke("sample", "input"))
		_, _ = nomagique.WriteArtifact(momentum, momentumInput)

		frame := make([]byte, 65536)
		readCount, err := momentum.Read(frame)

		So(err, ShouldEqual, io.EOF)
		So(readCount, ShouldBeGreaterThan, 0)

		outbound := datura.Acquire("momentum-outbound", datura.APPJSON)
		_, _ = outbound.Unpack(frame[:readCount])
		So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 0)
	})

	Convey("Given a repeated span after bootstrap", t, func() {
		momentum := NewMomentum(datura.Acquire("momentum-config", datura.APPJSON).Poke("sample", "input"))
		_, _ = nomagique.WriteArtifact(momentum, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))
		_, _ = momentum.Read(make([]byte, 65536))
		_, _ = nomagique.WriteArtifact(momentum, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))

		_, err := momentum.Read(make([]byte, 65536))

		So(err, ShouldNotBeNil)
	})

	Convey("Given warmed distinct samples", t, func() {
		momentum := NewMomentum(datura.Acquire("momentum-config", datura.APPJSON).Poke("sample", "input"))
		_, _ = nomagique.WriteArtifact(momentum, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))
		_, _ = momentum.Read(make([]byte, 65536))
		_, _ = nomagique.WriteArtifact(momentum, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 14))

		frame := make([]byte, 65536)
		readCount, err := momentum.Read(frame)

		So(err, ShouldEqual, io.EOF)
		So(readCount, ShouldBeGreaterThan, 0)

		outbound := datura.Acquire("momentum-outbound", datura.APPJSON)
		_, _ = outbound.Unpack(frame[:readCount])
		So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 1)
	})

	Convey("Given a non-finite sample", t, func() {
		momentum := NewMomentum(datura.Acquire("momentum-config", datura.APPJSON).Poke("sample", "input"))
		invalid := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", math.NaN())
		_, _ = nomagique.WriteArtifact(momentum, invalid)

		_, err := momentum.Read(make([]byte, 65536))

		So(err, ShouldNotBeNil)
	})
}

func TestMomentumWrite(t *testing.T) {
	Convey("Given a Momentum", t, func() {
		momentum := NewMomentum(datura.Acquire("momentum-config", datura.APPJSON).Poke("sample", "input"))

		Convey("When Write is called", func() {
			_, err := nomagique.WriteArtifact(momentum, momentumInput)
			So(err, ShouldBeNil)
		})
	})
}
