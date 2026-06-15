package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/tests"
)

func TestNewFracDiff(testingTB *testing.T) {
	Convey("Given NewFracDiff", testingTB, func() {
		fractional := NewFracDiff()

		Convey("It should return a usable stage", func() {
			So(fractional, ShouldNotBeNil)
			So(fractional.Ready, ShouldBeFalse)
		})
	})
}

func TestFracDiff_ObserveSample(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"bootstrap echo", []float64{10}, 10},
		{"collapsed repeat", []float64{10, 10, 10}, 10},
		{"second sample moves", []float64{0, 10}, 10},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			fractional := NewFracDiff()
			got := tests.RunObserveSampleSequence(fractional.ObserveSample, testCase.samples)

			Convey("It should derive the expected filtered value", func() {
				So(got, ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestFracDiff_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		fractional := NewFracDiff()

		Convey("It should return zero output", func() {
			So(observeInputs(fractional), ShouldEqual, 0)
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		fractional := NewFracDiff()
		_ = observeInputs(fractional, 10)

		Convey("It should leave output unchanged", func() {
			So(observeWithoutSample(fractional, 99), ShouldEqual, 10)
		})
	})

	Convey("Given a scalar plus work sample", testingTB, func() {
		fractional := NewFracDiff()
		_ = observeInputs(fractional, 0)

		Convey("It should match a single combined scalar", func() {
			withWork := observeWithCombinedWork(fractional, 5, 3)
			expect := NewFracDiff()
			_ = observeInputs(expect, 0)
			combined := observeInputs(expect, 8)

			So(withWork, ShouldEqual, combined)
		})
	})

	pathCases := []struct {
		name    string
		samples []float64
	}{
		{"volatile swing", []float64{10, 1, 20, 2, 15, 30}},
		{"monotone climb", []float64{1, 2, 3, 4, 5, 6, 7, 8}},
		{"adversarial spike", []float64{0, 0, 100, 0, 0}},
		{"negative range", []float64{-10, 10, -5, 5, 0}},
	}

	for _, testCase := range pathCases {
		testCase := testCase

		Convey("Given "+testCase.name+" via Observe scalars", testingTB, func() {
			sampleStage := NewFracDiff()
			scalarStage := NewFracDiff()

			sampleLast := tests.RunObserveSampleSequence(sampleStage.ObserveSample, testCase.samples)
			scalarLast := tests.RunObserveSampleSequence(scalarStage.ObserveSample, testCase.samples)

			Convey("It should match ObserveSample", func() {
				So(scalarLast, ShouldEqual, sampleLast)
			})
		})
	}
}

func TestFracDiff_ObserveSamples(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
	}{
		{"short batch", []float64{10, 20, 15}},
		{"long batch", []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			batch := NewFracDiff()
			sequential := NewFracDiff()

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

func TestFracDiff_Reset(testingTB *testing.T) {
	Convey("Given reset after warm stream", testingTB, func() {
		fractional := NewFracDiff()

		for _, sample := range []float64{10, 20, 5, 15, 30} {
			_ = fractional.ObserveSample(sample)
		}

		So(fractional.Reset(), ShouldBeNil)

		got := fractional.ObserveSample(99)

		Convey("It should bootstrap again", func() {
			So(got, ShouldEqual, 99)
			So(fractional.Ready, ShouldBeTrue)
			So(fractional.History, ShouldNotBeNil)
		})
	})
}

func BenchmarkFracDiff_ObserveSample(b *testing.B) {
	fractional := NewFracDiff()

	for _, sample := range []float64{10, 20, 15, 25} {
		_ = fractional.ObserveSample(sample)
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = fractional.ObserveSample(11)
	}
}
