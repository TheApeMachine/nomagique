package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/tests"
)

func TestNewVariance(testingTB *testing.T) {
	Convey("Given NewVariance", testingTB, func() {
		variance := NewVariance()

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
			variance := NewVariance()
			got := tests.RunObserveSampleSequence(variance.ObserveSample, testCase.samples)

			Convey("It should derive the expected variance", func() {
				So(got, ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestVariance_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		variance := NewVariance()

		Convey("It should return zero output", func() {
			So(observeInputs(variance), ShouldEqual, 0)
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		variance := NewVariance()
		_ = observeInputs(variance, 10)
		_ = observeInputs(variance, 20)

		Convey("It should leave output unchanged", func() {
			So(observeWithoutSample(variance, 99), ShouldEqual, 100)
		})
	})

	Convey("Given a scalar plus work sample", testingTB, func() {
		variance := NewVariance()
		_ = observeInputs(variance, 10)

		Convey("It should match a single combined scalar", func() {
			withWork := observeWithCombinedWork(variance, 5, 3)
			expect := NewVariance()
			_ = observeInputs(expect, 10)
			combined := observeInputs(expect, 8)

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
			sampleStage := NewVariance()
			scalarStage := NewVariance()

			sampleLast := tests.RunObserveSampleSequence(sampleStage.ObserveSample, testCase.samples)
			scalarLast := tests.RunObserveSampleSequence(scalarStage.ObserveSample, testCase.samples)

			Convey("It should match ObserveSample", func() {
				So(scalarLast, ShouldEqual, sampleLast)
			})
		})
	}
}

func TestVariance_ObserveSamples(testingTB *testing.T) {
	Convey("Given a sample batch", testingTB, func() {
		samples := []float64{10, 20, 15, 25}
		batch := NewVariance()
		sequential := NewVariance()

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
		variance := NewVariance()

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
	variance := NewVariance()
	_ = variance.ObserveSample(10)

	b.ReportAllocs()

	for b.Loop() {
		_ = variance.ObserveSample(11)
	}
}
