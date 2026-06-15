package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/tests"
)

func TestNewCompression(testingTB *testing.T) {
	Convey("Given NewCompression", testingTB, func() {
		compression := NewCompression[float64]()

		Convey("It should return a usable stage", func() {
			So(compression, ShouldNotBeNil)
			So(compression.Ready, ShouldBeFalse)
		})
	})
}

func TestCompression_ObserveSample(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"bootstrap zero", []float64{10}, 0},
		{"rise resets score", []float64{10, 15}, 0},
		{"half compression", []float64{10, 5}, 0.5},
		{"full compression to zero", []float64{10, 0}, 1},
		{"flat baseline", []float64{10, 10, 10}, 0},
		{"dip then rise", []float64{10, 5, 12}, 0},
		{"negative baseline rise", []float64{-10, -5}, 0},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			compression := NewCompression[float64]()
			got := tests.RunObserveSampleSequence(compression.ObserveSample, testCase.samples)

			Convey("It should derive the expected score", func() {
				So(got, ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestCompression_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		compression := NewCompression[float64]()

		Convey("It should return zero output", func() {
			So(compression.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		compression := NewCompression[float64]()
		_ = compression.Observe(core.Scalar[float64](10))
		_ = compression.Observe(core.Scalar[float64](5))
		stage := &tests.PipelineStage[float64]{Result: core.Scalar[float64](99)}

		Convey("It should leave output unchanged", func() {
			So(compression.Observe(stage), ShouldEqual, core.Scalar[float64](0.5))
		})
	})

	Convey("Given a scalar plus work sample", testingTB, func() {
		compression := NewCompression[float64]()
		_ = compression.Observe(core.Scalar[float64](10))

		Convey("It should match a single combined scalar", func() {
			withWork := compression.Observe(
				core.Scalar[float64](2),
				core.Scalar[float64](3),
			)
			expect := NewCompression[float64]()
			_ = expect.Observe(core.Scalar[float64](10))
			combined := expect.Observe(core.Scalar[float64](5))

			So(withWork, ShouldEqual, combined)
		})
	})

	pathCases := []struct {
		name    string
		samples []float64
	}{
		{"sawtooth dips", []float64{100, 50, 100, 25}},
		{"monotone decline", []float64{20, 15, 10, 5}},
	}

	for _, testCase := range pathCases {
		testCase := testCase

		Convey("Given "+testCase.name+" via Observe scalars", testingTB, func() {
			sampleStage := NewCompression[float64]()
			scalarStage := NewCompression[float64]()

			sampleLast := tests.RunObserveSampleSequence(sampleStage.ObserveSample, testCase.samples)
			scalarLast := tests.RunObserveScalarSequence(scalarStage.Observe, testCase.samples)

			Convey("It should match ObserveSample", func() {
				So(scalarLast, ShouldEqual, sampleLast)
			})
		})
	}
}

func TestCompression_ObserveSamples(testingTB *testing.T) {
	Convey("Given a sample batch after bootstrap", testingTB, func() {
		samples := []float64{5, 3, 8, 2}
		batch := NewCompression[float64]()
		sequential := NewCompression[float64]()

		_ = batch.Observe(core.Scalar[float64](10))
		_ = sequential.Observe(core.Scalar[float64](10))

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

func TestCompression_Reset(testingTB *testing.T) {
	Convey("Given reset after compression", testingTB, func() {
		compression := NewCompression[float64]()

		for _, sample := range []float64{10, 5, 3} {
			_ = compression.ObserveSample(sample)
		}

		So(compression.Reset(), ShouldBeNil)

		got := compression.ObserveSample(20)

		Convey("It should bootstrap again", func() {
			So(got, ShouldEqual, 0)
			So(compression.Baseline, ShouldEqual, 20)
		})
	})
}

func BenchmarkCompression_ObserveSample(b *testing.B) {
	compression := NewCompression[float64]()
	_ = compression.ObserveSample(10)

	b.ReportAllocs()

	for b.Loop() {
		_ = compression.ObserveSample(5)
	}
}
