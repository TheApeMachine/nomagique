package logic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestNewAnd(testingTB *testing.T) {
	Convey("Given an And gate", testingTB, func() {
		andGate := NewAnd[float64]()

		Convey("It should return a usable dynamic", func() {
			So(andGate, ShouldNotBeNil)
		})
	})
}

func TestAnd_Observe(testingTB *testing.T) {
	Convey("Given an And gate", testingTB, func() {
		andGate := NewAnd[float64]()

		Convey("It should emit truth when every input is truthy", func() {
			got := float64(andGate.Observe(scalarInputs[float64](1, 2, 0.5)...))
			So(got, ShouldEqual, 1)
		})

		Convey("It should emit false when any input is not truthy", func() {
			got := float64(andGate.Observe(scalarInputs[float64](1, 0, 0.5)...))
			So(got, ShouldEqual, 0)
		})
	})
}

func TestAnd_Reset(testingTB *testing.T) {
	Convey("Given an And gate with prior output", testingTB, func() {
		andGate := NewAnd[float64]()
		_ = andGate.Observe(scalarInputs[float64](1, 1)...)

		Convey("It should clear output on reset", func() {
			So(andGate.Reset(), ShouldBeNil)
			So(float64(andGate.Observe()), ShouldEqual, 0)
		})
	})
}

func TestOr_Observe(testingTB *testing.T) {
	Convey("Given an Or gate", testingTB, func() {
		orGate := NewOr[float64]()

		Convey("It should emit truth when any input is truthy", func() {
			got := float64(orGate.Observe(scalarInputs[float64](0, -1, 0.25)...))
			So(got, ShouldEqual, 1)
		})

		Convey("It should emit false when no input is truthy", func() {
			got := float64(orGate.Observe(scalarInputs[float64](0, -1, 0)...))
			So(got, ShouldEqual, 0)
		})
	})
}

func TestNot_Observe(testingTB *testing.T) {
	Convey("Given a Not gate", testingTB, func() {
		notGate := NewNot[float64]()

		Convey("It should invert truthy input", func() {
			got := float64(notGate.Observe(core.Scalar[float64](1)))
			So(got, ShouldEqual, 0)
		})

		Convey("It should invert falsy input", func() {
			got := float64(notGate.Observe(core.Scalar[float64](0)))
			So(got, ShouldEqual, 1)
		})
	})
}

func TestXor_Observe(testingTB *testing.T) {
	Convey("Given an Xor gate", testingTB, func() {
		xorGate := NewXor[float64]()

		Convey("It should emit truth for an odd truthy count", func() {
			got := float64(xorGate.Observe(scalarInputs[float64](1, 0, 0)...))
			So(got, ShouldEqual, 1)
		})

		Convey("It should emit false for an even truthy count", func() {
			got := float64(xorGate.Observe(scalarInputs[float64](1, 1, 0)...))
			So(got, ShouldEqual, 0)
		})
	})
}

func TestCompare_Observe(testingTB *testing.T) {
	Convey("Given a Compare stage", testingTB, func() {
		compare := NewCompare[float64]()

		Convey("It should emit positive sign when left exceeds right", func() {
			got := float64(compare.Observe(scalarInputs[float64](3, 1)...))
			So(got, ShouldEqual, 1)
		})

		Convey("It should emit negative sign when left is below right", func() {
			got := float64(compare.Observe(scalarInputs[float64](1, 3)...))
			So(got, ShouldEqual, -1)
		})

		Convey("It should emit zero when operands match", func() {
			got := float64(compare.Observe(scalarInputs[float64](2, 2)...))
			So(got, ShouldEqual, 0)
		})
	})
}

func TestSelect_Observe(testingTB *testing.T) {
	Convey("Given a Select stage", testingTB, func() {
		selectStage := NewSelect[float64]()

		Convey("It should pass the consequent when the condition is truthy", func() {
			got := float64(selectStage.Observe(scalarInputs[float64](1, 10, 20)...))
			So(got, ShouldEqual, 10)
		})

		Convey("It should pass the alternative when the condition is falsy", func() {
			got := float64(selectStage.Observe(scalarInputs[float64](0, 10, 20)...))
			So(got, ShouldEqual, 20)
		})
	})
}

func TestGate_Observe(testingTB *testing.T) {
	Convey("Given a Gate stage", testingTB, func() {
		gate := NewGate[float64]()

		Convey("It should pass the signal when enabled", func() {
			got := float64(gate.Observe(scalarInputs[float64](1, 42)...))
			So(got, ShouldEqual, 42)
		})

		Convey("It should block the signal when disabled", func() {
			got := float64(gate.Observe(scalarInputs[float64](0, 42)...))
			So(got, ShouldEqual, 0)
		})
	})
}

func TestMux_Observe(testingTB *testing.T) {
	Convey("Given a three-way Mux", testingTB, func() {
		mux := NewMux[float64](3)

		Convey("It should route to the selected value", func() {
			got := float64(mux.Observe(scalarInputs[float64](1, 10, 20, 30)...))
			So(got, ShouldEqual, 20)
		})

		Convey("It should retain output when the selector is out of range", func() {
			_ = mux.Observe(scalarInputs[float64](1, 10, 20, 30)...)
			got := float64(mux.Observe(scalarInputs[float64](5, 10, 20, 30)...))
			So(got, ShouldEqual, 20)
		})
	})
}

func TestFirstMatch_Observe(testingTB *testing.T) {
	Convey("Given a FirstMatch stage", testingTB, func() {
		firstMatch := NewFirstMatch[float64]()

		Convey("It should return the first matching consequent", func() {
			got := float64(firstMatch.Observe(scalarInputs[float64](0, 10, 1, 20, 99)...))
			So(got, ShouldEqual, 20)
		})

		Convey("It should return the default when no condition matches", func() {
			got := float64(firstMatch.Observe(scalarInputs[float64](0, 10, 0, 20, 99)...))
			So(got, ShouldEqual, 99)
		})
	})
}

func TestLatch_Observe(testingTB *testing.T) {
	Convey("Given a Latch stage", testingTB, func() {
		latch := NewLatch[float64]()

		Convey("It should hold the last captured signal", func() {
			_ = latch.Observe(scalarInputs[float64](1, 7)...)
			first := float64(latch.Observe(scalarInputs[float64](0, 3)...))
			second := float64(latch.Observe(scalarInputs[float64](1, 9)...))
			third := float64(latch.Observe(scalarInputs[float64](0, 3)...))

			So(first, ShouldEqual, 7)
			So(second, ShouldEqual, 9)
			So(third, ShouldEqual, 9)
		})
	})
}

func BenchmarkAnd_Observe(benchmark *testing.B) {
	andGate := NewAnd[float64]()
	inputs := scalarInputs[float64](1, 1, 1)

	for benchmark.Loop() {
		_ = andGate.Observe(inputs...)
	}
}

func BenchmarkSelect_Observe(benchmark *testing.B) {
	selectStage := NewSelect[float64]()
	inputs := scalarInputs[float64](1, 10, 20)

	for benchmark.Loop() {
		_ = selectStage.Observe(inputs...)
	}
}
