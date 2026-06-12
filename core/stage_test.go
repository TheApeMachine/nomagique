package core

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStageParser_Parse(testingTB *testing.T) {
	Convey("Given a stage parser", testingTB, func() {
		stageParser := NewStageParser()

		Convey("When parsing a single float input", func() {
			out, work, err := stageParser.Parse([]Number{Float64(9)})

			Convey("It should split out and empty work", func() {
				So(err, ShouldBeNil)
				So(out, ShouldEqual, 9)
				So(len(work), ShouldEqual, 0)
			})
		})

		Convey("When parsing multiple float inputs", func() {
			out, work, err := stageParser.Parse(
				[]Number{Float64(1), Float64(2), Float64(3)},
			)

			Convey("It should keep the first as out and rest as work", func() {
				So(err, ShouldBeNil)
				So(out, ShouldEqual, 1)
				So(work, ShouldResemble, []Float64{2, 3})
			})
		})
	})

	Convey("Given a non-float first input", testingTB, func() {
		stageParser := NewStageParser()

		Convey("When parsing", func() {
			_, _, err := stageParser.Parse([]Number{notFloatNumber{}})

			Convey("It should return zero", func() {
				So(err, ShouldEqual, ErrEmptyInputs)
			})
		})
	})

	Convey("Given a non-float work input", testingTB, func() {
		stageParser := NewStageParser()

		Convey("When parsing", func() {
			_, _, err := stageParser.Parse([]Number{Float64(1), notFloatNumber{}})

			Convey("It should return zero", func() {
				So(err, ShouldEqual, ErrEmptyInputs)
			})
		})
	})

	Convey("Given empty inputs", testingTB, func() {
		stageParser := NewStageParser()

		Convey("When parsing", func() {
			_, _, err := stageParser.Parse(nil)

			Convey("It should return zero", func() {
				So(err, ShouldEqual, ErrEmptyInputs)
			})
		})
	})

	Convey("Given a non-float input", testingTB, func() {
		stageParser := NewStageParser()

		Convey("When parsing", func() {
			_, _, err := stageParser.Parse([]Number{notFloatNumber{}})

			Convey("It should return zero", func() {
				So(err, ShouldEqual, ErrEmptyInputs)
			})
		})
	})
}

func BenchmarkStageParser_Parse(testingTB *testing.B) {
	stageParser := NewStageParser()
	inputs := []Number{Float64(1), Float64(2)}

	for testingTB.Loop() {
		_, _, _ = stageParser.Parse(inputs)
	}
}
