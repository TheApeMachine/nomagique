package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestPearson_Observe(testingTB *testing.T) {
	cases := []struct {
		name   string
		inputs []float64
		expect float64
	}{
		{
			name:   "perfect positive correlation",
			inputs: []float64{1, 2, 1, 2},
			expect: 1,
		},
		{
			name:   "linear streams",
			inputs: []float64{1, 2, 3, 4, 2, 4, 6, 8},
			expect: 1,
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			pearson := NewPearson[float64](nil)
			got := pearson.Observe(numberInputs(testCase.inputs...)...)

			Convey("It should return the expected correlation", func() {
				So(float64(got), ShouldEqual, testCase.expect)
			})
		})
	}

	Convey("Given empty Observe inputs", testingTB, func() {
		pearson := NewPearson[float64](nil)

		Convey("It should return zero output", func() {
			So(pearson.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given fewer than two inputs", testingTB, func() {
		pearson := NewPearson[float64](nil)
		got := pearson.Observe(core.Scalar[float64](1))

		Convey("It should return zero", func() {
			So(float64(got), ShouldEqual, 0)
		})
	})

	Convey("Given odd input count", testingTB, func() {
		pearson := NewPearson[float64](nil)
		got := pearson.Observe(numberInputs(1, 2, 3)...)

		Convey("It should return zero", func() {
			So(float64(got), ShouldEqual, 0)
		})
	})
}

func TestPearson_Reset(testingTB *testing.T) {
	Convey("Given an observed Pearson stage", testingTB, func() {
		pearson := NewPearson[float64](nil)
		_ = pearson.Observe(numberInputs(1, 2, 1, 2)...)

		So(pearson.Reset(), ShouldBeNil)

		Convey("It should clear output", func() {
			So(float64(pearson.Observe()), ShouldEqual, 0)
		})
	})
}

func BenchmarkPearson_Observe(testingTB *testing.B) {
	pearson := NewPearson[float64](nil)
	inputs := splitInputs(
		[]float64{1, 2, 3, 4},
		[]float64{2, 4, 6, 8},
	)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = pearson.Observe(inputs...)
	}
}
