package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestWeight(testingTB *testing.T) {
	Convey("Given Weight constructor", testingTB, func() {
		trustWeight := Weight(datura.Acquire("trust-weight-config", datura.APPJSON))

		Convey("It should return a usable dynamic", func() {
			So(trustWeight, ShouldNotBeNil)
		})
	})
}

func TestTrustWeight_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		trustWeight := Weight(datura.Acquire("trust-weight-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, trustWeight)

		So(err, ShouldBeNil)

		Convey("It should return zero output", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)
		})
	})

	Convey("Given a fresh trust weight", testingTB, func() {
		trustWeight := Weight(datura.Acquire("trust-weight-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(10, "sample").
			Poke(10, "paired")
		err := transport.NewFlipFlop(artifact, trustWeight)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return full trust", func() {
			So(got, ShouldEqual, 1)
		})
	})

	Convey("Given diverging outcomes", testingTB, func() {
		trustWeight := Weight(datura.Acquire("trust-weight-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(10, "sample").Poke(10, "paired")
		err := transport.NewFlipFlop(artifact, trustWeight)

		So(err, ShouldBeNil)

		artifact.Poke(20, "sample").Poke(30, "paired")
		err = transport.NewFlipFlop(artifact, trustWeight)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should reduce trust", func() {
			So(got, ShouldBeLessThan, 1)
		})
	})

	Convey("Given zero predicted", testingTB, func() {
		trustWeight := Weight(datura.Acquire("trust-weight-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(0, "sample").
			Poke(10, "paired")
		err := transport.NewFlipFlop(artifact, trustWeight)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should leave output at zero", func() {
			So(got, ShouldEqual, 0)
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		trustWeight := Weight(datura.Acquire("trust-weight-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(10, "sample").Poke(10, "paired")
		err := transport.NewFlipFlop(artifact, trustWeight)

		So(err, ShouldBeNil)

		before := datura.Peek[float64](artifact, "output", "value")

		fresh := datura.Acquire("test", datura.APPJSON)
		err = transport.NewFlipFlop(fresh, trustWeight)

		So(err, ShouldBeNil)

		Convey("It should leave output unchanged", func() {
			So(datura.Peek[float64](fresh, "output", "value"), ShouldEqual, before)
		})
	})
}

func TestTrustWeight_Reset(testingTB *testing.T) {
	Convey("Given trust weight with state", testingTB, func() {
		trustWeight := Weight(datura.Acquire("trust-weight-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(10, "sample").
			Poke(10, "paired")
		err := transport.NewFlipFlop(artifact, trustWeight)

		So(err, ShouldBeNil)
		So(trustWeight.Reset(), ShouldBeNil)

		fresh := datura.Acquire("test", datura.APPJSON)
		err = transport.NewFlipFlop(fresh, trustWeight)

		So(err, ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(trustWeight.state.Ready, ShouldBeFalse)
			So(datura.Peek[float64](fresh, "output", "value"), ShouldEqual, 0)
		})
	})
}

func BenchmarkWeight_Observe(testingTB *testing.B) {
	trustWeight := Weight(datura.Acquire("trust-weight-config-bench", datura.APPJSON))
	artifact := datura.Acquire("test", datura.APPJSON)

	artifact.Poke(10, "sample").Poke(10, "paired")
	_ = transport.NewFlipFlop(artifact, trustWeight)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact.Poke(10, "sample").Poke(11, "paired")
		_ = transport.NewFlipFlop(artifact, trustWeight)
	}
}

func BenchmarkWeight_ObserveSamples(testingTB *testing.B) {
	trustWeight := Weight(datura.Acquire("trust-weight-config-bench", datura.APPJSON))
	predicted := make([]float64, 1024)
	actual := make([]float64, len(predicted))
	out := make([]float64, len(predicted))

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		trustWeight.state.Reset()
		trustWeight.ObserveSamples(predicted, actual, out)
	}
}
