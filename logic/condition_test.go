package logic_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/logic"
)

func TestConditions(testingTB *testing.T) {
	Convey("Given typed comparisons", testingTB, func() {
		observation := logic.NewObservation(3)

		Convey("It should compare values against constants", func() {
			So(logic.GreaterThan{Right: logic.NewConstant(2)}.Match(observation), ShouldBeTrue)
			So(logic.LessThan{Right: logic.NewConstant(4)}.Match(observation), ShouldBeTrue)
			So(logic.Equal{Right: logic.NewConstant(3)}.Match(observation), ShouldBeTrue)
		})
	})

	Convey("Given compound conditions", testingTB, func() {
		observation := logic.NewObservation(3)
		above := logic.GreaterThan{Right: logic.NewConstant(2)}
		below := logic.LessThan{Right: logic.NewConstant(4)}

		Convey("It should evaluate boolean composition", func() {
			So(logic.And{above, below}.Match(observation), ShouldBeTrue)
			So(logic.Or{above}.Match(observation), ShouldBeTrue)
			So(logic.Not{Operand: above}.Match(observation), ShouldBeFalse)
			So(logic.Xor{above, below}.Match(observation), ShouldBeFalse)
			So(logic.True{}.Match(observation), ShouldBeTrue)
		})
	})
}
