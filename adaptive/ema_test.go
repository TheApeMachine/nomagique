package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/tests"
)

func TestNewEMA(testingTB *testing.T) {
	Convey("Given NewEMA", testingTB, func() {
		ema := NewEMA[float64]()

		Convey("It should return a usable stage", func() {
			So(ema, ShouldNotBeNil)
			So(ema.Ready, ShouldBeFalse)
		})
	})
}

func TestEMA_ObserveSample(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"bootstrap echo", []float64{10}, 10},
		{"collapsed repeat", []float64{10, 10, 10}, 10},
		{"unit step up", []float64{0, 10}, 10},
		{"full retrace", []float64{10, 20, 5}, 5},
		{"negative bootstrap", []float64{-5}, -5},
		{"negative to positive", []float64{-10, 10}, 10},
		{"zero bootstrap then rise", []float64{0, 5}, 5},
		{"oscillating range", []float64{1, 3, 1, 3}, 3},
		{"large magnitude", []float64{1e12, 1e12 + 1e6}, 1e12 + 1e6},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			ema := NewEMA[float64]()
			got := tests.RunObserveSampleSequence(ema.ObserveSample, testCase.samples)

			Convey("It should derive the expected value", func() {
				So(got, ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestEMA_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		ema := NewEMA[float64]()

		Convey("It should return zero output", func() {
			So(ema.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		ema := NewEMA[float64]()
		_ = ema.Observe(core.Scalar[float64](10))
		stage := &tests.PipelineStage[float64]{Result: core.Scalar[float64](99)}

		Convey("It should leave output unchanged", func() {
			So(ema.Observe(stage), ShouldEqual, core.Scalar[float64](10))
		})
	})

	Convey("Given a scalar plus work sample", testingTB, func() {
		ema := NewEMA[float64]()
		_ = ema.Observe(core.Scalar[float64](10))

		Convey("It should match a single combined scalar", func() {
			withWork := ema.Observe(
				core.Scalar[float64](5),
				core.Scalar[float64](3),
			)
			expect := NewEMA[float64]()
			_ = expect.Observe(core.Scalar[float64](10))
			combined := expect.Observe(core.Scalar[float64](8))

			So(withWork, ShouldEqual, combined)
		})
	})

	pathCases := []struct {
		name    string
		samples []float64
	}{
		{"monotone climb", []float64{1, 2, 3, 4, 5}},
		{"volatile swing", []float64{10, 1, 20, 2, 15}},
		{"negative series", []float64{-3, -1, -5, -2}},
	}

	for _, testCase := range pathCases {
		testCase := testCase

		Convey("Given "+testCase.name+" via Observe scalars", testingTB, func() {
			sampleStage := NewEMA[float64]()
			scalarStage := NewEMA[float64]()

			sampleLast := tests.RunObserveSampleSequence(sampleStage.ObserveSample, testCase.samples)
			scalarLast := tests.RunObserveScalarSequence(scalarStage.Observe, testCase.samples)

			Convey("It should match ObserveSample", func() {
				So(scalarLast, ShouldEqual, sampleLast)
			})
		})
	}
}

func TestEMA_ObserveSamples(testingTB *testing.T) {
	cases := []struct {
		name    string
		prefix  float64
		samples []float64
	}{
		{"after bootstrap", 10, []float64{11, 12, 13}},
		{"cold batch", 0, []float64{5, 10, 7}},
		{"adversarial swing", 0, []float64{100, 1, 50, 2}},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			batch := NewEMA[float64]()
			sequential := NewEMA[float64]()

			if testCase.prefix != 0 {
				_ = batch.Observe(core.Scalar[float64](testCase.prefix))
				_ = sequential.Observe(core.Scalar[float64](testCase.prefix))
			}

			batchOut := make([]float64, len(testCase.samples))
			batch.ObserveSamples(testCase.samples, batchOut)

			seqOut := make([]float64, len(testCase.samples))

			for index, sample := range testCase.samples {
				seqOut[index] = sequential.ObserveSample(sample)
			}

			Convey("It should match sequential ObserveSample", func() {
				for index := range testCase.samples {
					So(batchOut[index], ShouldEqual, seqOut[index])
				}
			})
		})
	}
}

func TestEMA_Reset(testingTB *testing.T) {
	cases := []struct {
		name         string
		beforeReset  []float64
		afterReset   float64
		expectOutput float64
	}{
		{"mid-stream", []float64{10, 20, 30}, 99, 99},
		{"after volatile run", []float64{1, 100, 2, 50}, 7, 7},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given reset after "+testCase.name, testingTB, func() {
			ema := NewEMA[float64]()

			for _, sample := range testCase.beforeReset {
				_ = ema.ObserveSample(sample)
			}

			So(ema.Reset(), ShouldBeNil)

			got := ema.ObserveSample(testCase.afterReset)

			Convey("It should bootstrap again", func() {
				So(got, ShouldEqual, testCase.expectOutput)
				So(ema.Ready, ShouldBeTrue)
			})
		})
	}
}

func BenchmarkEMA_ObserveSample(b *testing.B) {
	ema := NewEMA[float64]()
	_ = ema.ObserveSample(10)

	b.ReportAllocs()

	for b.Loop() {
		_ = ema.ObserveSample(11)
	}
}
