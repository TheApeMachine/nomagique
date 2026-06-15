package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewRLS(testingTB *testing.T) {
	Convey("Given NewRLS", testingTB, func() {
		stage, err := NewRLS(2, 1000)

		Convey("It should return a usable stage", func() {
			So(err, ShouldBeNil)
			So(stage, ShouldNotBeNil)
		})
	})
}

func TestRLS_Observe(testingTB *testing.T) {
	Convey("Given feature and target scalars", testingTB, func() {
		stage, err := NewRLS(1, 1000)
		So(err, ShouldBeNil)

		got := float64(observeWithWork(stage, 2, 4))

		Convey("It should derive a finite prediction", func() {
			So(got, ShouldBeGreaterThan, 0)
		})
	})
}

func TestRLS_Reset(testingTB *testing.T) {
	Convey("Given a reset RLS stage", testingTB, func() {
		stage, err := NewRLS(1, 1000)
		So(err, ShouldBeNil)
		_ = observeWithWork(stage, 1, 2)

		resetErr := stage.Reset()

		Convey("It should clear derived output", func() {
			So(resetErr, ShouldBeNil)
			So(float64(stage.output), ShouldEqual, 0)
		})
	})
}

func BenchmarkRLS_Observe(testingTB *testing.B) {
	stage, err := NewRLS(3, 1000)

	if err != nil {
		testingTB.Fatal(err)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = observeInputs(stage, 1, 2, 3, 6)
	}
}
