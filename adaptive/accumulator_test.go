package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/tests"
)

func TestNewAccumulator(testingTB *testing.T) {
	Convey("Given NewAccumulator", testingTB, func() {
		accumulator := NewAccumulator()

		Convey("It should return a usable stage", func() {
			So(accumulator, ShouldNotBeNil)
			So(accumulator.Level, ShouldEqual, 0)
		})
	})
}

func TestAccumulator_ObserveSample(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"single positive", []float64{10}, 10},
		{"zero holds level", []float64{10, 0}, 10},
		{"positive run", []float64{10, 5, 3}, 18},
		{"negative integration", []float64{-5, 3}, -2},
		{"alternating signs", []float64{10, -4, 2, -1}, 7},
		{"large magnitude", []float64{1e9, 1e9}, 2e9},
		{"tiny increments", []float64{1e-12, 1e-12}, 2e-12},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			accumulator := NewAccumulator()
			got := tests.RunObserveSampleSequence(accumulator.ObserveSample, testCase.samples)

			Convey("It should derive the expected level", func() {
				So(got, ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestAccumulator_Observe(testingTB *testing.T) {
	Convey("Given empty pipeline input", testingTB, func() {
		accumulator := NewAccumulator()

		Convey("It should return zero output", func() {
			So(pipelineSample(accumulator, 0), ShouldEqual, 0)
		})
	})

	Convey("Given a retained accumulator", testingTB, func() {
		accumulator := NewAccumulator()
		_ = pipelineSample(accumulator, 10)

		Convey("It should continue integrating", func() {
			So(pipelineSample(accumulator, 5), ShouldEqual, 15)
		})
	})

	pathCases := []struct {
		name    string
		samples []float64
	}{
		{"random walk", []float64{1, -2, 3, -1, 4}},
		{"all zeros", []float64{0, 0, 0, 0}},
	}

	for _, testCase := range pathCases {
		testCase := testCase

		Convey("Given "+testCase.name+" via pipeline", testingTB, func() {
			sampleStage := NewAccumulator()
			pipelineStage := NewAccumulator()

			sampleLast := tests.RunObserveSampleSequence(sampleStage.ObserveSample, testCase.samples)
			pipelineLast := pipelineSamples(pipelineStage, testCase.samples...)

			Convey("It should match ObserveSample", func() {
				So(pipelineLast, ShouldEqual, sampleLast)
			})
		})
	}
}

func TestAccumulator_ObserveSamples(testingTB *testing.T) {
	Convey("Given a sample batch", testingTB, func() {
		samples := []float64{1, 2, 3, -1}
		batch := NewAccumulator()
		sequential := NewAccumulator()

		batchOut := make([]float64, len(samples))
		batch.ObserveSamples(samples, batchOut)

		seqOut := make([]float64, len(samples))

		for index, sample := range samples {
			seqOut[index] = sequential.ObserveSample(sample)
		}

		Convey("It should match sequential ObserveSample", func() {
			for index := range samples {
				So(batchOut[index], ShouldEqual, seqOut[index])
			}
		})
	})
}

func TestAccumulator_Reset(testingTB *testing.T) {
	Convey("Given reset after integration", testingTB, func() {
		accumulator := NewAccumulator()

		for _, sample := range []float64{10, 20, -5} {
			_ = accumulator.ObserveSample(sample)
		}

		So(accumulator.Reset(), ShouldBeNil)

		got := accumulator.ObserveSample(7)

		Convey("It should start from zero level", func() {
			So(got, ShouldEqual, 7)
			So(accumulator.Level, ShouldEqual, 7)
		})
	})
}

func BenchmarkAccumulator_ObserveSample(b *testing.B) {
	accumulator := NewAccumulator()

	b.ReportAllocs()

	for b.Loop() {
		_ = accumulator.ObserveSample(1)
	}
}
