package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/causal"
	"github.com/theapemachine/nomagique/core"
)

func TestPearl_Observe(testingTB *testing.T) {
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

		config := causal.LadderConfig{
			TreatmentNormal:   2,
			ControlsNormal:    []int{0, 1},
			TreatmentInverted: 1,
			ControlsInverted:  []int{0},
			ConditionLeft:     1,
			ConditionRight:    2,
			MinHistory:        12,
		}

		streams := [][]float64{nodeZero, nodeOne, nodeTwo, nodeThree}
		ladder := NewPearl[float64](3, config, streams, core.Scalar[float64](0), nil)

		intervention := ladder.Observe()

		Convey("It should return a positive intervention effect", func() {
			So(float64(intervention), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkPearl_Observe(testingTB *testing.B) {
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

	config := causal.LadderConfig{
		TreatmentNormal:   2,
		ControlsNormal:    []int{0, 1},
		TreatmentInverted: 1,
		ControlsInverted:  []int{0},
		ConditionLeft:     1,
		ConditionRight:    2,
		MinHistory:        12,
	}

	streams := [][]float64{nodeZero, nodeOne, nodeTwo, nodeThree}
	ladder := NewPearl[float64](3, config, streams, core.Scalar[float64](0), nil)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = ladder.Observe()
	}
}
