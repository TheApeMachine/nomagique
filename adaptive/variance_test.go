package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/tests"
)

func TestNewVariance(testingTB *testing.T) {
	Convey("Given NewVariance", testingTB, func() {
		variance := NewVariance[float64]()

		Convey("It should return a usable stage", func() {
			So(variance, ShouldNotBeNil)
			So(variance.Ready, ShouldBeFalse)
		})
	})
}

func TestVariance_ObserveSample(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"bootstrap zero", []float64{10}, 0},
		{"unit step variance", []float64{10, 20}, 100},
		{"collapsed repeat", []float64{10, 10, 10}, 0},
		{"symmetric swing", []float64{0, 10, 0}, 100},
		{"negative bootstrap", []float64{-5, 5}, 100},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			variance := NewVariance[float64]()
			got := tests.RunObserveSampleSequence(variance.ObserveSample, testCase.samples)

			Convey("It should derive the expected variance", func() {
				So(got, ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestVariance_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		variance := NewVariance[float64]()

		Convey("It should return zero output", func() {
			So(variance.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		variance := NewVariance[float64]()
		_ = variance.Observe(core.Scalar[float64](10))
		_ = variance.Observe(core.Scalar[float64](20))
		stage := &tests.PipelineStage[float64]{Result: core.Scalar[float64](99)}

		Convey("It should leave output unchanged", func() {
			So(variance.Observe(stage), ShouldEqual, core.Scalar[float64](100))
		})
	})

	Convey("Given a scalar plus work sample", testingTB, func() {
		variance := NewVariance[float64]()
		_ = variance.Observe(core.Scalar[float64](10))

		Convey("It should match a single combined scalar", func() {
			withWork := variance.Observe(
				core.Scalar[float64](5),
				core.Scalar[float64](3),
			)
			expect := NewVariance[float64]()
			_ = expect.Observe(core.Scalar[float64](10))
			combined := expect.Observe(core.Scalar[float64](8))

			So(withWork, ShouldEqual, combined)
		})
	})

	pathCases := []struct {
		name    string
		samples []float64
	}{
		{"volatile swing", []float64{10, 1, 20, 2, 15}},
		{"trend", []float64{1, 2, 3, 4, 5, 6}},
	}

	for _, testCase := range pathCases {
		testCase := testCase

		Convey("Given "+testCase.name+" via Observe scalars", testingTB, func() {
			sampleStage := NewVariance[float64]()
			scalarStage := NewVariance[float64]()

			sampleLast := tests.RunObserveSampleSequence(sampleStage.ObserveSample, testCase.samples)
			scalarLast := tests.RunObserveScalarSequence(scalarStage.Observe, testCase.samples)

			Convey("It should match ObserveSample", func() {
				So(scalarLast, ShouldEqual, sampleLast)
			})
		})
	}
}

func TestVariance_ObserveSamples(testingTB *testing.T) {
	Convey("Given a sample batch", testingTB, func() {
		samples := []float64{10, 20, 15, 25}
		batch := NewVariance[float64]()
		sequential := NewVariance[float64]()

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

func TestVariance_Reset(testingTB *testing.T) {
	Convey("Given reset after warm stream", testingTB, func() {
		variance := NewVariance[float64]()

		for _, sample := range []float64{10, 20, 30} {
			_ = variance.ObserveSample(sample)
		}

		So(variance.Reset(), ShouldBeNil)

		got := variance.ObserveSample(99)

		Convey("It should bootstrap again", func() {
			So(got, ShouldEqual, 0)
			So(variance.Ready, ShouldBeTrue)
		})
	})
}

func BenchmarkVariance_ObserveSample(b *testing.B) {
	variance := NewVariance[float64]()
	_ = variance.ObserveSample(10)

	b.ReportAllocs()

	for b.Loop() {
		_ = variance.ObserveSample(11)
	}
}
