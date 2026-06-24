package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestWeight(testingTB *testing.T) {
	Convey("Given Weight constructor", testingTB, func() {
		trustWeight := Weight(pairConfig("trust-weight-config"))

		Convey("It should return a usable dynamic", func() {
			So(trustWeight, ShouldNotBeNil)
		})
	})
}

func TestTrustWeightRead(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		trustWeight := Weight(pairConfig("trust-weight-config"))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, trustWeight)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a fresh trust weight", testingTB, func() {
		trustWeight := Weight(pairConfig("trust-weight-config"))
		artifact := pairWire(datura.Acquire("test", datura.APPJSON), 10, 10)
		err := transport.NewFlipFlop(artifact, trustWeight)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return full trust", func() {
			So(got, ShouldEqual, 1)
		})
	})

	Convey("Given diverging outcomes", testingTB, func() {
		trustWeight := Weight(pairConfig("trust-weight-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact = pairWire(artifact, 10, 10)
		err := transport.NewFlipFlop(artifact, trustWeight)

		So(err, ShouldBeNil)

		artifact = pairWire(artifact, 20, 30)
		err = transport.NewFlipFlop(artifact, trustWeight)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should reduce trust", func() {
			So(got, ShouldBeLessThan, 1)
		})
	})

	Convey("Given zero predicted", testingTB, func() {
		trustWeight := Weight(pairConfig("trust-weight-config"))
		artifact := pairWire(datura.Acquire("test", datura.APPJSON), 0, 10)
		err := transport.NewFlipFlop(artifact, trustWeight)

		Convey("It should return a parse error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		trustWeight := Weight(pairConfig("trust-weight-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact = pairWire(artifact, 10, 10)
		err := transport.NewFlipFlop(artifact, trustWeight)

		So(err, ShouldBeNil)

		fresh := datura.Acquire("test", datura.APPJSON)
		err = transport.NewFlipFlop(fresh, trustWeight)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestTrustWeight_Reset(testingTB *testing.T) {
	Convey("Given trust weight with state", testingTB, func() {
		trustWeight := Weight(pairConfig("trust-weight-config"))
		artifact := pairWire(datura.Acquire("test", datura.APPJSON), 10, 10)
		err := transport.NewFlipFlop(artifact, trustWeight)

		So(err, ShouldBeNil)

		fields := weightStateFromArtifact(trustWeight.artifact)
		fields.Count = 0
		pokeWeightState(trustWeight.artifact, &fields, 0)

		Convey("It should clear derived state", func() {
			So(datura.Peek[float64](trustWeight.artifact, "output", "count"), ShouldEqual, 0)
		})

		fresh := pairWire(datura.Acquire("test", datura.APPJSON), 10, 10)
		err = transport.NewFlipFlop(fresh, trustWeight)

		So(err, ShouldBeNil)

		Convey("It should observe again after reset", func() {
			So(datura.Peek[float64](trustWeight.artifact, "output", "count"), ShouldEqual, 1)
			So(datura.Peek[float64](fresh, "output", "value"), ShouldEqual, 1)
		})
	})
}

func BenchmarkTrustWeightRead(testingTB *testing.B) {
	trustWeight := Weight(pairConfig("trust-weight-config-bench"))
	artifact := datura.Acquire("test", datura.APPJSON)

	artifact = pairWire(artifact, 10, 10)
	_ = transport.NewFlipFlop(artifact, trustWeight)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact = pairWire(artifact, 10, 11)
		_ = transport.NewFlipFlop(artifact, trustWeight)
	}
}
