package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestPositiveOnlyRead(t *testing.T) {
	Convey("Given a positive-only gate", t, func() {
		config := datura.Acquire("positive-only-config", datura.APPJSON).
			Poke("precursor", "outputKey").
			Poke(1.0, "positiveOnly")

		stage := NewPositiveOnly(config)
		artifact := datura.Acquire("positive-only-test", datura.APPJSON)
		artifact.Poke("output", "root")
		artifact.Poke([]string{"precursor"}, "inputs")
		artifact.Merge("output", map[string]any{"precursor": -2.0})

		err := transport.NewFlipFlop(artifact, stage)

		Convey("It should clamp negative scores to zero", func() {
			So(err, ShouldBeNil)
			So(datura.Peek[string](artifact, "root"), ShouldEqual, "output")
			So(datura.Peek[float64](artifact, "output", "precursor"), ShouldEqual, 0)
		})
	})

	Convey("Given missing outputKey config", t, func() {
		stage := NewPositiveOnly(datura.Acquire("positive-only-missing", datura.APPJSON))
		artifact := ScalarWire(datura.Acquire("positive-only-test", datura.APPJSON), "sample", 1.0)

		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldNotBeNil)
	})
}

func BenchmarkPositiveOnlyRead(b *testing.B) {
	config := datura.Acquire("positive-only-bench", datura.APPJSON).
		Poke("precursor", "outputKey").
		Poke(1.0, "positiveOnly")

	stage := NewPositiveOnly(config)
	artifact := datura.Acquire("positive-only-bench-test", datura.APPJSON)
	artifact.Poke("output", "root")
	artifact.Poke([]string{"precursor"}, "inputs")
	artifact.Merge("output", map[string]any{"precursor": 1.5})

	b.ReportAllocs()

	for b.Loop() {
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
