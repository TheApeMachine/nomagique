package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/tests"
)

func TestNewCompression(testingTB *testing.T) {
	Convey("Given NewCompression", testingTB, func() {
		compression := NewCompression()

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
			compression := NewCompression()
			got := tests.RunObserveSampleSequence(compression.ObserveSample, testCase.samples)

			Convey("It should derive the expected score", func() {
				So(got, ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestCompression_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		compression := NewCompression()

		Convey("It should return zero output", func() {
			So(observeInputs(compression), ShouldEqual, 0)
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		compression := NewCompression()
		_ = observeInputs(compression, 10)
		_ = observeInputs(compression, 5)

		Convey("It should leave output unchanged", func() {
			So(observeWithoutSample(compression, 99), ShouldEqual, 0.5)
		})
	})

	Convey("Given a scalar plus work sample", testingTB, func() {
		compression := NewCompression()
		_ = observeInputs(compression, 10)

		Convey("It should match a single combined scalar", func() {
			withWork := observeWithCombinedWork(compression, 2, 3)
			expect := NewCompression()
			_ = observeInputs(expect, 10)
			combined := observeInputs(expect, 5)

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
			sampleStage := NewCompression()
			scalarStage := NewCompression()

			sampleLast := tests.RunObserveSampleSequence(sampleStage.ObserveSample, testCase.samples)
			scalarLast := tests.RunObserveSampleSequence(scalarStage.ObserveSample, testCase.samples)

			Convey("It should match ObserveSample", func() {
				So(scalarLast, ShouldEqual, sampleLast)
			})
		})
	}
}

func TestCompression_ObserveSamples(testingTB *testing.T) {
	Convey("Given a sample batch after bootstrap", testingTB, func() {
		samples := []float64{5, 3, 8, 2}
		batch := NewCompression()
		sequential := NewCompression()

		_ = observeInputs(batch, 10)
		_ = observeInputs(sequential, 10)

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
		compression := NewCompression()

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
	compression := NewCompression()
	_ = compression.ObserveSample(10)

	b.ReportAllocs()

	for b.Loop() {
		_ = compression.ObserveSample(5)
	}
}
