package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func buildRegimeTable(testingTB *testing.T) NodeTable {
	testingTB.Helper()

	rows := make([][]float64, 16)

	for index := range rows {
		rows[index] = []float64{
			float64(index) * 0.1,
			float64(index) * 0.2,
			float64(index) * 0.05,
			float64(index) * 0.3,
		}
	}

	table, err := NewNodeTable(rows, 3, 12)

	So(err, ShouldBeNil)

	return table
}

func TestSelectRoles(testingTB *testing.T) {
	config := LadderConfig{
		TreatmentNormal:   2,
		ControlsNormal:    []int{0, 1},
		TreatmentInverted: 1,
		ControlsInverted:  []int{0, 2},
		ContagionBreak:    0.8,
	}

	Convey("Given low contagion", testingTB, func() {
		table := buildRegimeTable(testingTB)
		roles, inverted, condition := SelectRoles(table, 0.1, config)

		Convey("It should select normal roles", func() {
			So(inverted, ShouldBeFalse)
			So(roles.Label, ShouldEqual, "normal")
			So(roles.Treatment, ShouldEqual, config.TreatmentNormal)
			So(roles.Controls, ShouldResemble, config.ControlsNormal)
			So(condition, ShouldBeGreaterThanOrEqualTo, 0)
		})
	})

	Convey("Given contagion above break", testingTB, func() {
		table := buildRegimeTable(testingTB)
		roles, inverted, _ := SelectRoles(table, 0.95, config)

		Convey("It should invert roles", func() {
			So(inverted, ShouldBeTrue)
			So(roles.Label, ShouldEqual, "inverted")
			So(roles.Treatment, ShouldEqual, config.TreatmentInverted)
			So(roles.Controls, ShouldResemble, config.ControlsInverted)
		})
	})
}

func TestRoles_Predictors(testingTB *testing.T) {
	Convey("Given role assignment", testingTB, func() {
		roles := Roles{Treatment: 2, Controls: []int{0, 1}}

		Convey("It should append treatment after controls", func() {
			So(roles.Predictors(), ShouldResemble, []int{0, 1, 2})
		})
	})
}

func TestSelectRolesWithTracker(testingTB *testing.T) {
	config := LadderConfig{
		TreatmentNormal:   2,
		ControlsNormal:    []int{0},
		TreatmentInverted: 1,
		ControlsInverted:  []int{0},
		ContagionBreak:    0.5,
	}

	Convey("Given nil tracker", testingTB, func() {
		table := buildRegimeTable(testingTB)
		roles, inverted, _ := SelectRolesWithTracker(table, 0.9, config, nil, 16)

		Convey("It should pass through raw inversion", func() {
			So(inverted, ShouldBeTrue)
			So(roles.Label, ShouldEqual, "inverted")
		})
	})
}
