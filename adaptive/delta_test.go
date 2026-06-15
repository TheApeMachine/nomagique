package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/tests"
)

func TestNewDelta(testingTB *testing.T) {
	Convey("Given NewDelta", testingTB, func() {
		delta := NewDelta[float64]()

		Convey("It should return a usable stage", func() {
			So(delta, ShouldNotBeNil)
			So(delta.Ready, ShouldBeFalse)
		})
	})
}

func TestDelta_ObserveSample(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"bootstrap zero", []float64{10}, 0},
		{"unit step up", []float64{0, 10}, 1},
		{"collapsed repeat", []float64{10, 10, 10}, 0},
		{"half span move", []float64{10, 20}, 1},
		{"retrace within span", []float64{10, 20, 15}, 0.5},
		{"negative range", []float64{-10, 10}, 1},
		{"zero span bootstrap", []float64{0, 0}, 0},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			delta := NewDelta[float64]()
			got := tests.RunObserveSampleSequence(delta.ObserveSample, testCase.samples)

			Convey("It should derive the expected value", func() {
				So(got, ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestDelta_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		delta := NewDelta[float64]()

		Convey("It should return zero output", func() {
			So(delta.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		delta := NewDelta[float64]()
		_ = delta.Observe(core.Scalar[float64](10))
		stage := &tests.PipelineStage[float64]{Result: core.Scalar[float64](99)}

		Convey("It should leave output unchanged", func() {
			So(delta.Observe(stage), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given a scalar plus work sample", testingTB, func() {
		delta := NewDelta[float64]()
		_ = delta.Observe(core.Scalar[float64](0))

		Convey("It should match a single combined scalar", func() {
			withWork := delta.Observe(
				core.Scalar[float64](5),
				core.Scalar[float64](3),
			)
			expect := NewDelta[float64]()
			_ = expect.Observe(core.Scalar[float64](0))
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
	}

	for _, testCase := range pathCases {
		testCase := testCase

		Convey("Given "+testCase.name+" via Observe scalars", testingTB, func() {
			sampleStage := NewDelta[float64]()
			scalarStage := NewDelta[float64]()

			sampleLast := tests.RunObserveSampleSequence(sampleStage.ObserveSample, testCase.samples)
			scalarLast := tests.RunObserveScalarSequence(scalarStage.Observe, testCase.samples)

			Convey("It should match ObserveSample", func() {
				So(scalarLast, ShouldEqual, sampleLast)
			})
		})
	}
}

func TestDelta_ObserveSamples(testingTB *testing.T) {
	cases := []struct {
		name    string
		prefix  float64
		samples []float64
	}{
		{"after bootstrap", 10, []float64{11, 12, 13}},
		{"cold batch", 0, []float64{5, 10, 7}},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			batch := NewDelta[float64]()
			sequential := NewDelta[float64]()

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

func TestDelta_Reset(testingTB *testing.T) {
	Convey("Given reset after a warm stream", testingTB, func() {
		delta := NewDelta[float64]()

		for _, sample := range []float64{10, 20, 30} {
			_ = delta.ObserveSample(sample)
		}

		So(delta.Reset(), ShouldBeNil)

		got := delta.ObserveSample(99)

		Convey("It should bootstrap again", func() {
			So(got, ShouldEqual, 0)
			So(delta.Ready, ShouldBeTrue)
		})
	})
}

func BenchmarkDelta_ObserveSample(b *testing.B) {
	delta := NewDelta[float64]()
	_ = delta.ObserveSample(10)

	b.ReportAllocs()

	for b.Loop() {
		_ = delta.ObserveSample(11)
	}
}
