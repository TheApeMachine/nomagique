package core

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestErrNilNumber(testingTB *testing.T) {
	Convey("Given the nil number error", testingTB, func() {
		Convey("It should be available for failed composition", func() {
			So(ErrNilNumber, ShouldNotBeNil)
		})
	})
}

func TestErrEmptyInputs(testingTB *testing.T) {
	Convey("Given the empty inputs error", testingTB, func() {
		Convey("It should be available for failed observation", func() {
			So(ErrEmptyInputs, ShouldNotBeNil)
		})
	})
}

func TestErrZeroPredicted(testingTB *testing.T) {
	Convey("Given the zero predicted error", testingTB, func() {
		Convey("It should be available for invalid learning pairs", func() {
			So(ErrZeroPredicted, ShouldNotBeNil)
		})
	})
}
