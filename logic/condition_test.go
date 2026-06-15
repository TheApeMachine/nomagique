package logic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTrue_Match(testingTB *testing.T) {
	Convey("Given a literal True condition", testingTB, func() {
		condition := True{Operand: true}

		Convey("It should always match", func() {
			So(matchConditionWithoutSample(condition), ShouldBeTrue)
		})
	})
}

func TestGreaterThan_Match(testingTB *testing.T) {
	Convey("Given a carried signal above Right", testingTB, func() {
		condition := GreaterThan{
			Right: NewConstant(2),
		}

		Convey("It should match when the boundary exceeds Right", func() {
			So(matchCondition(condition, 3), ShouldBeTrue)
		})
	})
}

func TestAnd_Match(testingTB *testing.T) {
	Convey("Given an And condition", testingTB, func() {
		condition := And{
			True{Operand: true},
			GreaterThan{
				Right: NewConstant(2),
			},
		}

		Convey("It should match when every operand matches", func() {
			So(matchCondition(condition, 3), ShouldBeTrue)
		})
	})
}

func TestOr_Match(testingTB *testing.T) {
	Convey("Given an Or condition", testingTB, func() {
		condition := Or{
			True{Operand: false},
			GreaterThan{
				Right: NewConstant(2),
			},
		}

		Convey("It should match when any operand matches", func() {
			So(matchCondition(condition, 3), ShouldBeTrue)
		})
	})
}

func TestNot_Match(testingTB *testing.T) {
	Convey("Given a Not condition", testingTB, func() {
		condition := Not{
			Operand: True{Operand: false},
		}

		Convey("It should invert the nested condition", func() {
			So(matchConditionWithoutSample(condition), ShouldBeTrue)
		})
	})
}

func TestXor_Match(testingTB *testing.T) {
	Convey("Given an Xor condition", testingTB, func() {
		condition := Xor{
			True{Operand: true},
			True{Operand: false},
			True{Operand: false},
		}

		Convey("It should match on an odd truthy count", func() {
			So(matchConditionWithoutSample(condition), ShouldBeTrue)
		})
	})
}
