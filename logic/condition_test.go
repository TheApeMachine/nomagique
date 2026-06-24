package logic_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/logic"
)

func matchCondition(condition logic.Condition, sample float64) bool {
	artifact := scalarWire(datura.Acquire("test", datura.APPJSON), sample)

	return condition.Match(artifact)
}

func matchConditionWithoutSample(condition logic.Condition) bool {
	artifact := datura.Acquire("test", datura.APPJSON)

	return condition.Match(artifact)
}

func TestTrue_Match(testingTB *testing.T) {
	Convey("Given a literal True condition", testingTB, func() {
		condition := logic.True{Operand: true}

		Convey("It should always match", func() {
			So(matchConditionWithoutSample(condition), ShouldBeTrue)
		})
	})
}

func TestGreaterThan_Match(testingTB *testing.T) {
	Convey("Given a carried signal above Right", testingTB, func() {
		condition := logic.GreaterThan{
			Right: constantStage(2),
		}

		Convey("It should match when the boundary exceeds Right", func() {
			So(matchCondition(condition, 3), ShouldBeTrue)
		})
	})
}

func TestAnd_Match(testingTB *testing.T) {
	Convey("Given an And condition", testingTB, func() {
		condition := logic.And{
			logic.True{Operand: true},
			logic.GreaterThan{
				Right: constantStage(2),
			},
		}

		Convey("It should match when every operand matches", func() {
			So(matchCondition(condition, 3), ShouldBeTrue)
		})
	})
}

func TestOr_Match(testingTB *testing.T) {
	Convey("Given an Or condition", testingTB, func() {
		condition := logic.Or{
			logic.True{Operand: false},
			logic.GreaterThan{
				Right: constantStage(2),
			},
		}

		Convey("It should match when any operand matches", func() {
			So(matchCondition(condition, 3), ShouldBeTrue)
		})
	})
}

func TestNot_Match(testingTB *testing.T) {
	Convey("Given a Not condition", testingTB, func() {
		condition := logic.Not{
			Operand: logic.True{Operand: false},
		}

		Convey("It should invert the nested condition", func() {
			So(matchConditionWithoutSample(condition), ShouldBeTrue)
		})
	})
}

func TestXor_Match(testingTB *testing.T) {
	Convey("Given an Xor condition", testingTB, func() {
		condition := logic.Xor{
			logic.True{Operand: true},
			logic.True{Operand: false},
			logic.True{Operand: false},
		}

		Convey("It should match on an odd truthy count", func() {
			So(matchConditionWithoutSample(condition), ShouldBeTrue)
		})
	})
}
