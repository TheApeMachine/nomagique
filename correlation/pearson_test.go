package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestPearson_Observe(testingTB *testing.T) {
	Convey("Given a Pearson dynamic", testingTB, func() {
		pearson := NewPearson(nil)

		Convey("When inputs are split into two equal streams", func() {
			value := pearson.Observe(
				core.Float64(1), core.Float64(2),
				core.Float64(1), core.Float64(2),
			)

			Convey("It should correlate the first and second halves", func() {
				So(value, ShouldEqual, core.Float64(1))
			})
		})

		Convey("When fewer than two inputs are provided", func() {
			value := pearson.Observe(core.Float64(1))

			Convey("It should return zero", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})

		Convey("When inputs cannot split into equal halves", func() {
			value := pearson.Observe(
				core.Float64(1), core.Float64(2), core.Float64(3),
			)

			Convey("It should return zero", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

func BenchmarkPearson_Observe(testingTB *testing.B) {
	pearson := NewPearson(nil)
	inputs := []core.Number{
		core.Float64(1), core.Float64(2), core.Float64(3), core.Float64(4),
		core.Float64(2), core.Float64(4), core.Float64(6), core.Float64(8),
	}

	for testingTB.Loop() {
		_ = pearson.Observe(inputs...)
	}
}
