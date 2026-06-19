package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
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
			artifact := datura.Acquire("test", datura.APPJSON).Poke(testCase.inputs, "batch")
			err := transport.NewFlipFlop(artifact, pearson)

			So(err, ShouldBeNil)

			got := datura.Peek[float64](artifact, "output", "value")

			Convey("It should return the expected correlation", func() {
				So(got, ShouldEqual, testCase.expect)
			})
		})
	}

	Convey("Given empty Observe inputs", testingTB, func() {
		pearson := NewPearson(nil)
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, pearson)

		So(err, ShouldBeNil)

		Convey("It should return zero output", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)
		})
	})

	Convey("Given fewer than two inputs", testingTB, func() {
		pearson := NewPearson(nil)
		artifact := datura.Acquire("test", datura.APPJSON).Poke(1, "sample")
		err := transport.NewFlipFlop(artifact, pearson)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return zero", func() {
			So(got, ShouldEqual, 0)
		})
	})

	Convey("Given odd input count", testingTB, func() {
		pearson := NewPearson(nil)
		artifact := datura.Acquire("test", datura.APPJSON).Poke([]float64{1, 2, 3}, "batch")
		err := transport.NewFlipFlop(artifact, pearson)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return zero", func() {
			So(got, ShouldEqual, 0)
		})
	})
}

func TestPearson_Reset(testingTB *testing.T) {
	Convey("Given an observed Pearson stage", testingTB, func() {
		pearson := NewPearson(nil)
		artifact := datura.Acquire("test", datura.APPJSON).Poke([]float64{1, 2, 1, 2}, "batch")
		err := transport.NewFlipFlop(artifact, pearson)

		So(err, ShouldBeNil)
		So(pearson.Reset(), ShouldBeNil)

		fresh := datura.Acquire("test", datura.APPJSON)
		err = transport.NewFlipFlop(fresh, pearson)

		So(err, ShouldBeNil)

		Convey("It should clear output", func() {
			So(datura.Peek[float64](fresh, "output", "value"), ShouldEqual, 0)
		})
	})
}

func BenchmarkPearson_Observe(testingTB *testing.B) {
	pearson := NewPearson(nil)
	artifact := datura.Acquire("test", datura.APPJSON)

	for testingTB.Loop() {
		artifact.Poke([]float64{1, 2, 3, 4, 2, 4, 6, 8}, "batch")
		_ = transport.NewFlipFlop(artifact, pearson)
	}
}
