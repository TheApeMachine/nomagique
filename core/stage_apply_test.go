package core

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStage_contract(testingTB *testing.T) {
	Convey("Given a stage with Apply", testingTB, func() {
		var stage Stage = fastStage{}

		Convey("It should transform pipeline work without interface fan-out", func() {
			result, err := stage.Apply(Float64(1), []Float64{2, 3})

			So(err, ShouldBeNil)
			So(result, ShouldEqual, Float64(3))
		})
	})

	Convey("Given a failing stage", testingTB, func() {
		var stage Stage = errStage{err: errStageFailed}

		Convey("It should surface Apply errors", func() {
			_, err := stage.Apply(Float64(1), []Float64{2})

			So(err, ShouldEqual, errStageFailed)
		})
	})
}
