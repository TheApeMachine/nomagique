package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
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

		artifact := datura.Acquire("test", datura.APPJSON).Poke([]float64{2, 4}, "batch")
		err = transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should derive a finite prediction", func() {
			So(got, ShouldBeGreaterThan, 0)
		})
	})
}

func TestRLS_Reset(testingTB *testing.T) {
	Convey("Given a reset RLS stage", testingTB, func() {
		stage, err := NewRLS(1, 1000)
		So(err, ShouldBeNil)

		artifact := datura.Acquire("test", datura.APPJSON).Poke([]float64{1, 2}, "batch")
		err = transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		resetErr := stage.Reset()

		Convey("It should clear derived output", func() {
			So(resetErr, ShouldBeNil)
			So(stage.output, ShouldEqual, 0)
		})
	})
}

func BenchmarkRLS_Observe(testingTB *testing.B) {
	stage, err := NewRLS(3, 1000)

	if err != nil {
		testingTB.Fatal(err)
	}

	artifact := datura.Acquire("test", datura.APPJSON)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact.Poke([]float64{1, 2, 3, 6}, "batch")
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
