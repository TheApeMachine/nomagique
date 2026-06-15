package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/causal"
)

func TestPearl_Readings(testingTB *testing.T) {
	Convey("Given a Pearl ladder", testingTB, func() {
		ladder := NewPearl(
			3,
			causal.LadderConfig{MinHistory: 12},
			nil,
			newFixedScore(0),
			nil,
		)

		Convey("It should publish composable ladder score sources", func() {
			So(ladder.UpliftReading(), ShouldNotBeNil)
			So(ladder.ContagionReading(), ShouldNotBeNil)
			So(ladder.AssociationReading(), ShouldNotBeNil)
			So(ladder.InterventionReading(), ShouldNotBeNil)
		})
	})
}

func TestPearl_Read(testingTB *testing.T) {
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
		nodes := nodeRingFromStreams(streams)
		ladder := NewPearl(3, config, nodes, newFixedScore(0), nil)

		intervention := observeInputs(ladder)

		Convey("It should return a positive intervention effect", func() {
			So(intervention, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkPearl_Read(testingTB *testing.B) {
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
	nodes := nodeRingFromStreams(streams)
	ladder := NewPearl(3, config, nodes, newFixedScore(0), nil)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = observeInputs(ladder)
	}
}
