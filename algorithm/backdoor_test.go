package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBackdoor_Observe(testingTB *testing.T) {
	Convey("Given aligned node streams with causal structure", testingTB, func() {
		nodeZero := make([]float64, 16)
		nodeOne := make([]float64, 16)
		nodeTwo := make([]float64, 16)
		nodeThree := make([]float64, 16)

		for index := range nodeZero {
			nodeZero[index] = float64(index) * 0.1
			nodeOne[index] = float64(index) * 0.2
			nodeTwo[index] = float64(index) * 0.5
			nodeThree[index] = float64(index) * 0.05
		}

		streams := [][]float64{nodeZero, nodeOne, nodeTwo, nodeThree}
		backdoor := NewBackdoor(3, 2, []int{0, 1}, streams, 12)
		effect := observeInputs(backdoor)

		Convey("It should return a finite backdoor effect", func() {
			So(float64(effect), ShouldNotEqual, 0)
			So(float64(backdoor.Association()), ShouldNotEqual, 0)
			So(float64(backdoor.ConditionNumber()), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkBackdoor_Observe(testingTB *testing.B) {
	nodeZero := make([]float64, 16)
	nodeOne := make([]float64, 16)
	nodeTwo := make([]float64, 16)
	nodeThree := make([]float64, 16)

	for index := range nodeZero {
		nodeZero[index] = float64(index) * 0.1
		nodeOne[index] = float64(index) * 0.2
		nodeTwo[index] = float64(index) * 0.5
		nodeThree[index] = float64(index) * 0.05
	}

	streams := [][]float64{nodeZero, nodeOne, nodeTwo, nodeThree}
	backdoor := NewBackdoor(3, 2, []int{0, 1}, streams, 12)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = observeInputs(backdoor)
	}
}
