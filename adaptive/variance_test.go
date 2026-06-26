package adaptive

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func TestVarianceRead(t *testing.T) {
	Convey("Given a Variance", t, func() {
		variance := NewVariance(datura.Acquire("variance-config", datura.APPJSON).Poke("sample", "input"))

		Convey("When the first sample arrives", func() {
			_, err := nomagique.WriteArtifact(variance, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))

			So(err, ShouldBeIn, nil, io.EOF)

			_, err = variance.Read(make([]byte, 65536))

			So(err, ShouldNotBeNil)
		})

		Convey("When warmed up with distinct samples", func() {
			_, _ = nomagique.WriteArtifact(variance, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))
			_, _ = variance.Read(make([]byte, 65536))
			_, _ = nomagique.WriteArtifact(variance, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 22))
			_, _ = variance.Read(make([]byte, 65536))
			_, _ = nomagique.WriteArtifact(variance, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 30))
			_, _ = variance.Read(make([]byte, 65536))
			artifact := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 40)
			_, _ = nomagique.WriteArtifact(variance, artifact)

			frame := make([]byte, 65536)
			readCount, err := variance.Read(frame)

			So(err, ShouldBeIn, nil, io.EOF)
			So(readCount, ShouldBeGreaterThan, 0)

			err = nomagique.RoundTripArtifact(artifact, variance)

			So(err, ShouldBeIn, nil, io.EOF)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})
}

func TestVarianceWrite(t *testing.T) {
	Convey("Given a Variance", t, func() {
		variance := NewVariance(datura.Acquire("variance-config", datura.APPJSON).Poke("sample", "input"))

		Convey("When Write is called", func() {
			_, err := nomagique.WriteArtifact(variance, ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10))
			So(err, ShouldBeIn, nil, io.EOF)
		})
	})
}
