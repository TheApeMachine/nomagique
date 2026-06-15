package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/tests"
)

func TestNewMomentum(testingTB *testing.T) {
	Convey("Given NewMomentum", testingTB, func() {
		momentum := NewMomentum()

		Convey("It should return a usable stage", func() {
			So(momentum, ShouldNotBeNil)
			So(momentum.Ready, ShouldBeFalse)
		})
	})
}

func TestMomentum_ObserveSample(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"bootstrap zero", []float64{10}, 0},
		{"unit rise", []float64{10, 20}, 1},
		{"unit fall", []float64{20, 10}, -1},
		{"collapsed repeat", []float64{10, 10, 10}, 0},
		{"mid span move", []float64{0, 10, 5}, -0.5},
		{"negative to positive", []float64{-10, 10}, 1},
		{"zero span after bootstrap", []float64{5, 5}, 0},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			momentum := NewMomentum()
			got := tests.RunObserveSampleSequence(momentum.ObserveSample, testCase.samples)

			Convey("It should derive the expected signed move", func() {
				So(got, ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestMomentum_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		momentum := NewMomentum()

		Convey("It should return zero output", func() {
			So(observeInputs(momentum), ShouldEqual, 0)
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		momentum := NewMomentum()
		_ = observeInputs(momentum, 10)
		_ = observeInputs(momentum, 20)

		Convey("It should leave output unchanged", func() {
			So(observeWithoutSample(momentum, 99), ShouldEqual, 1)
		})
	})

	Convey("Given a scalar plus work sample", testingTB, func() {
		momentum := NewMomentum()
		_ = observeInputs(momentum, 0)

		Convey("It should match a single combined scalar", func() {
			withWork := observeWithCombinedWork(momentum, 5, 3)
			expect := NewMomentum()
			_ = observeInputs(expect, 0)
			combined := observeInputs(expect, 8)

			So(withWork, ShouldEqual, combined)
		})
	})

	pathCases := []struct {
		name    string
		samples []float64
	}{
		{"volatile swing", []float64{10, 1, 20, 2, 15}},
		{"trend up", []float64{1, 2, 3, 4, 5}},
	}

	for _, testCase := range pathCases {
		testCase := testCase

		Convey("Given "+testCase.name+" via Observe scalars", testingTB, func() {
			sampleStage := NewMomentum()
			scalarStage := NewMomentum()

			sampleLast := tests.RunObserveSampleSequence(sampleStage.ObserveSample, testCase.samples)
			scalarLast := tests.RunObserveSampleSequence(scalarStage.ObserveSample, testCase.samples)

			Convey("It should match ObserveSample", func() {
				So(scalarLast, ShouldEqual, sampleLast)
			})
		})
	}
}

func TestMomentum_ObserveSamples(testingTB *testing.T) {
	Convey("Given a sample batch", testingTB, func() {
		samples := []float64{10, 20, 5, 15}
		batch := NewMomentum()
		sequential := NewMomentum()

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

func TestMomentum_Reset(testingTB *testing.T) {
	Convey("Given reset after warm stream", testingTB, func() {
		momentum := NewMomentum()

		for _, sample := range []float64{10, 20, 5} {
			_ = momentum.ObserveSample(sample)
		}

		So(momentum.Reset(), ShouldBeNil)

		got := momentum.ObserveSample(99)

		Convey("It should bootstrap again", func() {
			So(got, ShouldEqual, 0)
			So(momentum.Ready, ShouldBeTrue)
		})
	})
}

func BenchmarkMomentum_ObserveSample(b *testing.B) {
	momentum := NewMomentum()
	_ = momentum.ObserveSample(10)

	b.ReportAllocs()

	for b.Loop() {
		_ = momentum.ObserveSample(11)
	}
}
