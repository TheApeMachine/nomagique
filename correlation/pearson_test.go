package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
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
			pearson := NewPearson(nil)
			got := observeInputs(pearson, testCase.inputs...)

			Convey("It should return the expected correlation", func() {
				So(float64(got), ShouldEqual, testCase.expect)
			})
		})
	}

	Convey("Given empty Observe inputs", testingTB, func() {
		pearson := NewPearson(nil)

		Convey("It should return zero output", func() {
			So(observeInputs(pearson), ShouldEqual, 0)
		})
	})

	Convey("Given fewer than two inputs", testingTB, func() {
		pearson := NewPearson(nil)
		got := observeInputs(pearson, 1)

		Convey("It should return zero", func() {
			So(float64(got), ShouldEqual, 0)
		})
	})

	Convey("Given odd input count", testingTB, func() {
		pearson := NewPearson(nil)
		got := observeInputs(pearson, 1, 2, 3)

		Convey("It should return zero", func() {
			So(float64(got), ShouldEqual, 0)
		})
	})
}

func TestPearson_Reset(testingTB *testing.T) {
	Convey("Given an observed Pearson stage", testingTB, func() {
		pearson := NewPearson(nil)
		_ = observeInputs(pearson, 1, 2, 1, 2)

		So(pearson.Reset(), ShouldBeNil)

		Convey("It should clear output", func() {
			So(float64(observeInputs(pearson)), ShouldEqual, 0)
		})
	})
}

func BenchmarkPearson_Observe(testingTB *testing.B) {
	pearson := NewPearson(nil)
	for testingTB.Loop() {
		_ = observeSplit(pearson,
			[]float64{1, 2, 3, 4},
			[]float64{2, 4, 6, 8},
		)
	}
}
