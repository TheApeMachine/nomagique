package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/tests"
)

func TestNewRange(testingTB *testing.T) {
	Convey("Given NewRange", testingTB, func() {
		extent := NewRange[float64]()

		Convey("It should return a usable stage", func() {
			So(extent, ShouldNotBeNil)
			So(extent.Ready, ShouldBeFalse)
		})
	})
}

func TestRange_ObserveSample(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"bootstrap zero", []float64{10}, 0},
		{"expand up", []float64{10, 20}, 10},
		{"expand down", []float64{20, 10}, 10},
		{"collapsed repeat", []float64{10, 10, 10}, 0},
		{"widening envelope", []float64{5, 15, 0, 20}, 20},
		{"negative span", []float64{-10, 10}, 20},
		{"single point after bootstrap", []float64{7, 7}, 0},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			extent := NewRange[float64]()
			got := tests.RunObserveSampleSequence(extent.ObserveSample, testCase.samples)

			Convey("It should derive the expected span", func() {
				So(got, ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestRange_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		extent := NewRange[float64]()

		Convey("It should return zero output", func() {
			So(extent.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		extent := NewRange[float64]()
		_ = extent.Observe(core.Scalar[float64](10))
		_ = extent.Observe(core.Scalar[float64](20))
		stage := &tests.PipelineStage[float64]{Result: core.Scalar[float64](99)}

		Convey("It should leave output unchanged", func() {
			So(extent.Observe(stage), ShouldEqual, core.Scalar[float64](10))
		})
	})

	Convey("Given a scalar plus work sample", testingTB, func() {
		extent := NewRange[float64]()
		_ = extent.Observe(core.Scalar[float64](10))

		Convey("It should match a single combined scalar", func() {
			withWork := extent.Observe(
				core.Scalar[float64](5),
				core.Scalar[float64](3),
			)
			expect := NewRange[float64]()
			_ = expect.Observe(core.Scalar[float64](10))
			combined := expect.Observe(core.Scalar[float64](8))

			So(withWork, ShouldEqual, combined)
		})
	})

	pathCases := []struct {
		name    string
		samples []float64
	}{
		{"random walk", []float64{10, 1, 20, 2, 15, 0}},
		{"monotone", []float64{1, 2, 3, 4, 5}},
	}

	for _, testCase := range pathCases {
		testCase := testCase

		Convey("Given "+testCase.name+" via Observe scalars", testingTB, func() {
			sampleStage := NewRange[float64]()
			scalarStage := NewRange[float64]()

			sampleLast := tests.RunObserveSampleSequence(sampleStage.ObserveSample, testCase.samples)
			scalarLast := tests.RunObserveScalarSequence(scalarStage.Observe, testCase.samples)

			Convey("It should match ObserveSample", func() {
				So(scalarLast, ShouldEqual, sampleLast)
			})
		})
	}
}

func TestRange_ObserveSamples(testingTB *testing.T) {
	Convey("Given a sample batch", testingTB, func() {
		samples := []float64{10, 20, 5, 15}
		batch := NewRange[float64]()
		sequential := NewRange[float64]()

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

func TestRange_Reset(testingTB *testing.T) {
	Convey("Given reset after warm stream", testingTB, func() {
		extent := NewRange[float64]()

		for _, sample := range []float64{10, 20, 5} {
			_ = extent.ObserveSample(sample)
		}

		So(extent.Reset(), ShouldBeNil)

		got := extent.ObserveSample(99)

		Convey("It should bootstrap again", func() {
			So(got, ShouldEqual, 0)
			So(extent.Ready, ShouldBeTrue)
		})
	})
}

func BenchmarkRange_ObserveSample(b *testing.B) {
	extent := NewRange[float64]()
	_ = extent.ObserveSample(10)

	b.ReportAllocs()

	for b.Loop() {
		_ = extent.ObserveSample(11)
	}
}
