package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestNewCUSUM(testingTB *testing.T) {
	Convey("Given CUSUM constructor", testingTB, func() {
		changeSum := NewCUSUM(cusumConfig("cusum-config"))

		Convey("It should return a usable dynamic", func() {
			So(changeSum, ShouldNotBeNil)
		})
	})
}

func TestCUSUM_Read(testingTB *testing.T) {
	Convey("Given empty inbound wire", testingTB, func() {
		changeSum := NewCUSUM(cusumConfig("cusum-config"))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, changeSum)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a warmed change sum", testingTB, func() {
		changeSum := NewCUSUM(cusumConfig("cusum-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		scalarWire(artifact, "sample", 10)
		err := transport.NewFlipFlop(artifact, changeSum)

		So(err, ShouldBeNil)

		scalarWire(artifact, "sample", 25)
		err = transport.NewFlipFlop(artifact, changeSum)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should accumulate evidence", func() {
			So(got, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given sequential scalar samples", testingTB, func() {
		changeSum := NewCUSUM(cusumConfig("cusum-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		scalarWire(artifact, "sample", 10)
		err := transport.NewFlipFlop(artifact, changeSum)

		So(err, ShouldBeNil)

		scalarWire(artifact, "sample", 8)
		err = transport.NewFlipFlop(artifact, changeSum)

		So(err, ShouldBeNil)

		withWork := datura.Peek[float64](artifact, "output", "value")

		combined := NewCUSUM(cusumConfig("cusum-config-combined"))
		reference := datura.Acquire("test", datura.APPJSON)

		scalarWire(reference, "sample", 10)
		err = transport.NewFlipFlop(reference, combined)

		So(err, ShouldBeNil)

		scalarWire(reference, "sample", 8)
		err = transport.NewFlipFlop(reference, combined)

		So(err, ShouldBeNil)

		direct := datura.Peek[float64](reference, "output", "value")

		Convey("It should match a single combined scalar", func() {
			So(withWork, ShouldEqual, direct)
		})
	})

	Convey("Given reset after observation", testingTB, func() {
		changeSum := NewCUSUM(cusumConfig("cusum-config"))
		artifact := scalarWire(datura.Acquire("test", datura.APPJSON), "sample", 10)

		err := transport.NewFlipFlop(artifact, changeSum)

		So(err, ShouldBeNil)

		resetArtifact := datura.Acquire("test", datura.APPJSON).Poke(1, "reset")
		err = transport.NewFlipFlop(resetArtifact, changeSum)

		So(err, ShouldNotBeNil)

		Convey("It should clear derived state", func() {
			So(datura.Peek[float64](changeSum.artifact, "output", "ready"), ShouldEqual, 0)
			So(datura.Peek[float64](changeSum.artifact, "output", "value"), ShouldEqual, 0)
		})
	})
}

func TestCUSUMState_Observe(testingTB *testing.T) {
	Convey("Given a fresh CUSUM state", testingTB, func() {
		state := CUSUMState{}

		Convey("When bootstrapping", func() {
			evidence := state.Observe(10)

			Convey("It should initialize without evidence", func() {
				So(state.Ready, ShouldBeTrue)
				So(state.Target, ShouldEqual, 10)
				So(evidence, ShouldEqual, 0)
			})
		})
	})

	Convey("Given CUSUM history", testingTB, func() {
		state := CUSUMState{}
		_ = state.Observe(10)
		evidence := state.Observe(25)

		Convey("It should accumulate positive evidence", func() {
			So(evidence, ShouldBeGreaterThan, 0)
			So(state.Positive, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given a downward move", testingTB, func() {
		state := CUSUMState{}
		_ = state.Observe(10)
		evidence := state.Observe(5)

		Convey("It should reset positive accumulation", func() {
			So(evidence, ShouldEqual, 0)
			So(state.Positive, ShouldEqual, 0)
		})
	})

	Convey("Given sustained upward drift", testingTB, func() {
		state := CUSUMState{}
		_ = state.Observe(10)
		_ = state.Observe(12)
		evidence := state.Observe(15)

		Convey("It should keep accumulating positive evidence", func() {
			So(evidence, ShouldBeGreaterThan, 0)
			So(state.Positive, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given zero span after bootstrap", testingTB, func() {
		state := CUSUMState{}
		_ = state.Observe(7)
		evidence := state.Observe(7)

		Convey("It should hold zero evidence without span", func() {
			So(evidence, ShouldEqual, 0)
			So(state.Positive, ShouldEqual, 0)
		})
	})
}

func TestCUSUMState_ObserveSamples(testingTB *testing.T) {
	Convey("Given samples", testingTB, func() {
		state := CUSUMState{}
		samples := []float64{10, 25}
		out := make([]float64, len(samples))

		Convey("When observing in batch", func() {
			state.ObserveSamples(samples, out)

			Convey("It should match sequential observation", func() {
				expect := CUSUMState{}
				for index, sample := range samples {
					So(out[index], ShouldEqual, expect.Observe(sample))
				}
			})
		})
	})
}

func BenchmarkCUSUM_Read(testingTB *testing.B) {
	changeSum := NewCUSUM(cusumConfig("cusum-config-bench"))
	artifact := datura.Acquire("test", datura.APPJSON)

	scalarWire(artifact, "sample", 10)
	_ = transport.NewFlipFlop(artifact, changeSum)
	scalarWire(artifact, "sample", 10.5)
	_ = transport.NewFlipFlop(artifact, changeSum)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		scalarWire(artifact, "sample", 10.5)
		_ = transport.NewFlipFlop(artifact, changeSum)
	}
}

func BenchmarkCUSUMState_Observe(testingTB *testing.B) {
	state := CUSUMState{}
	_ = state.Observe(10)

	for testingTB.Loop() {
		_ = state.Observe(10.5)
	}
}
