package correlation

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestHayashiYoshida_Observe(testingTB *testing.T) {
	Convey("Given proportional async streams", testingTB, func() {
		hayashi := NewHayashiYoshida[float64](nil, time.Second)
		got := hayashi.Observe(numberInputs(
			0, 100,
			1, 110,
			0, 50,
			1, 55,
		)...)

		Convey("It should estimate correlation near one", func() {
			So(float64(got), ShouldAlmostEqual, 1, 1e-9)
		})
	})

	Convey("Given empty Observe inputs", testingTB, func() {
		hayashi := NewHayashiYoshida[float64](nil, time.Second)

		Convey("It should return zero output", func() {
			So(hayashi.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given fewer than two inputs", testingTB, func() {
		hayashi := NewHayashiYoshida[float64](nil, time.Second)
		got := hayashi.Observe(core.Scalar[float64](1))

		Convey("It should return zero", func() {
			So(float64(got), ShouldEqual, 0)
		})
	})

	Convey("Given odd input count", testingTB, func() {
		hayashi := NewHayashiYoshida[float64](nil, time.Second)
		got := hayashi.Observe(numberInputs(0, 100, 1)...)

		Convey("It should return zero", func() {
			So(float64(got), ShouldEqual, 0)
		})
	})

	Convey("Given a half that is not time-value pairs", testingTB, func() {
		hayashi := NewHayashiYoshida[float64](nil, time.Second)
		got := hayashi.Observe(numberInputs(
			0, 100,
			0, 50, 1, 55,
		)...)

		Convey("It should return zero", func() {
			So(float64(got), ShouldEqual, 0)
		})
	})
}

func TestHayashiYoshida_Reset(testingTB *testing.T) {
	Convey("Given an observed Hayashi stage", testingTB, func() {
		hayashi := NewHayashiYoshida[float64](nil, time.Second)
		_ = hayashi.Observe(numberInputs(0, 100, 1, 110, 0, 50, 1, 55)...)

		So(hayashi.Reset(), ShouldBeNil)

		Convey("It should clear output", func() {
			So(float64(hayashi.Observe()), ShouldEqual, 0)
		})
	})
}

func BenchmarkHayashiYoshida_Observe(testingTB *testing.B) {
	hayashi := NewHayashiYoshida[float64](nil, time.Second)
	inputs := numberInputs(
		0, 100,
		1, 110,
		2, 121,
		3, 133.1,
		0, 50,
		1, 55,
		2, 60.5,
		3, 66.55,
	)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = hayashi.Observe(inputs...)
	}
}
