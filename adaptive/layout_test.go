package adaptive

import (
	"testing"
	"unsafe"
)

func TestEMAState_layout(testingTB *testing.T) {
	var state EMAState

	if unsafe.Offsetof(state.Value) != 0 {
		testingTB.Fatal("Value offset")
	}

	if unsafe.Offsetof(state.Prev) != 8 {
		testingTB.Fatal("Prev offset")
	}

	if unsafe.Offsetof(state.Min) != 16 {
		testingTB.Fatal("Min offset")
	}

	if unsafe.Offsetof(state.Max) != 24 {
		testingTB.Fatal("Max offset")
	}

	if unsafe.Offsetof(state.Rate) != 32 {
		testingTB.Fatal("Rate offset")
	}

	if unsafe.Offsetof(state.Ready) != 40 {
		testingTB.Fatal("Ready offset")
	}
}

func TestAccumulatorState_layout(testingTB *testing.T) {
	var state AccumulatorState

	if unsafe.Offsetof(state.Level) != 0 {
		testingTB.Fatal("Level offset")
	}
}

func TestCompressionState_layout(testingTB *testing.T) {
	var state CompressionState

	if unsafe.Offsetof(state.Baseline) != 0 {
		testingTB.Fatal("Baseline offset")
	}

	if unsafe.Offsetof(state.Ready) != 8 {
		testingTB.Fatal("Ready offset")
	}
}

func TestFracDiffState_layout(testingTB *testing.T) {
	var state FracDiffState

	if unsafe.Offsetof(state.Prev) != 0 {
		testingTB.Fatal("Prev offset")
	}

	if unsafe.Offsetof(state.Min) != 8 {
		testingTB.Fatal("Min offset")
	}

	if unsafe.Offsetof(state.Max) != 16 {
		testingTB.Fatal("Max offset")
	}

	if unsafe.Offsetof(state.Order) != 24 {
		testingTB.Fatal("Order offset")
	}

	if unsafe.Offsetof(state.Ready) != 32 {
		testingTB.Fatal("Ready offset")
	}
}

func TestDeltaState_layout(testingTB *testing.T) {
	var state DeltaState

	if unsafe.Offsetof(state.Prev) != 0 {
		testingTB.Fatal("Prev offset")
	}

	if unsafe.Offsetof(state.Min) != 8 {
		testingTB.Fatal("Min offset")
	}

	if unsafe.Offsetof(state.Max) != 16 {
		testingTB.Fatal("Max offset")
	}

	if unsafe.Offsetof(state.Ready) != 24 {
		testingTB.Fatal("Ready offset")
	}
}
