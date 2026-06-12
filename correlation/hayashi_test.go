package correlation

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestHayashiYoshida_Observe(testingTB *testing.T) {
	Convey("Given a Hayashi-Yoshida dynamic", testingTB, func() {
		hayashi := NewHayashiYoshida(nil, time.Second)

		Convey("When both streams move proportionally on overlapping intervals", func() {
			value := hayashi.Observe(
				core.Float64(0), core.Float64(100),
				core.Float64(1), core.Float64(110),
				core.Float64(0), core.Float64(50),
				core.Float64(1), core.Float64(55),
			)

			Convey("It should estimate correlation near one", func() {
				So(float64(value), ShouldAlmostEqual, 1, 1e-9)
			})
		})

		Convey("When fewer than two inputs are provided", func() {
			value := hayashi.Observe(core.Float64(1))

			Convey("It should return zero", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})

		Convey("When inputs cannot split into equal halves", func() {
			value := hayashi.Observe(
				core.Float64(0), core.Float64(100), core.Float64(1),
			)

			Convey("It should return zero", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})

		Convey("When a half is not encoded as time-value pairs", func() {
			value := hayashi.Observe(
				core.Float64(0), core.Float64(100),
				core.Float64(0), core.Float64(50), core.Float64(1), core.Float64(55),
			)

			Convey("It should return zero", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

func BenchmarkHayashiYoshida_Observe(testingTB *testing.B) {
	hayashi := NewHayashiYoshida(nil, time.Second)
	inputs := []core.Number{
		core.Float64(0), core.Float64(100),
		core.Float64(1), core.Float64(110),
		core.Float64(2), core.Float64(121),
		core.Float64(3), core.Float64(133.1),
		core.Float64(0), core.Float64(50),
		core.Float64(1), core.Float64(55),
		core.Float64(2), core.Float64(60.5),
		core.Float64(3), core.Float64(66.55),
	}

	for testingTB.Loop() {
		_ = hayashi.Observe(inputs...)
	}
}
