package logic_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/logic"
)

func TestCircuit(t *testing.T) {
	Convey("Given a typed circuit", t, func() {
		circuit := logic.NewCircuit(logic.Rules{
			{
				Condition: logic.GreaterThan{Right: logic.NewConstant(10)},
				Then:      logic.NewConstant(99),
			},
			{
				Condition: logic.True{},
				Then:      logic.NewConstant(7),
			},
		})

		output, err := circuit.Measure(logic.NewObservation(3))

		Convey("It should route through the first matching rule", func() {
			So(err, ShouldBeNil)
			So(output.Values, ShouldResemble, []float64{7})
		})

		output, err = circuit.Measure(logic.NewObservation(13))

		Convey("It should stop after a higher-priority match", func() {
			So(err, ShouldBeNil)
			So(output.Values, ShouldResemble, []float64{99})
		})
	})
}
