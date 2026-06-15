package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/tests"
)

func TestNewZScore(testingTB *testing.T) {
	Convey("Given NewZScore", testingTB, func() {
		surprise := NewZScore()

		Convey("It should return a usable stage", func() {
			So(surprise, ShouldNotBeNil)
			So(surprise.Ready, ShouldBeFalse)
		})
	})
}

func TestZScore_ObserveSample(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"bootstrap zero", []float64{10}, 0},
		{"collapsed repeat", []float64{10, 10, 10}, 0},
		{"second sample unit z-score", []float64{10, 20}, 1},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			surprise := NewZScore()
			got := tests.RunObserveSampleSequence(surprise.ObserveSample, testCase.samples)

			Convey("It should derive the expected score", func() {
				So(got, ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestZScore_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		surprise := NewZScore()

		Convey("It should return zero output", func() {
			So(observeInputs(surprise), ShouldEqual, 0)
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		surprise := NewZScore()
		_ = observeInputs(surprise, 10)

		Convey("It should leave output unchanged", func() {
			So(observeWithoutSample(surprise, 99), ShouldEqual, 0)
		})
	})

	Convey("Given an anchor mean on the second input", testingTB, func() {
		surprise := NewZScore()
		_ = observeInputs(surprise, 10)
		_ = observeInputs(surprise, 20)
		_ = observeInputs(surprise, 30)

		Convey("It should use the anchor instead of adapting mean", func() {
			withAnchor := observeWithWork(surprise, 40, 10)
			expect := NewZScore()

			for _, sample := range []float64{10, 20, 30} {
				_ = observeInputs(expect, sample)
			}

			anchor := observeWithWork(expect, 40, 10)

			So(withAnchor, ShouldEqual, anchor)
		})
	})

	pathCases := []struct {
		name    string
		samples []float64
	}{
		{"volatile swing", []float64{10, 1, 20, 2, 15, 30}},
		{"trend", []float64{1, 2, 3, 4, 5, 6, 7}},
		{"adversarial spike", []float64{0, 0, 0, 100, 0}},
	}

	for _, testCase := range pathCases {
		testCase := testCase

		Convey("Given "+testCase.name+" via Observe scalars", testingTB, func() {
			sampleStage := NewZScore()
			scalarStage := NewZScore()

			sampleLast := tests.RunObserveSampleSequence(sampleStage.ObserveSample, testCase.samples)
			scalarLast := tests.RunObserveSampleSequence(scalarStage.ObserveSample, testCase.samples)

			Convey("It should match ObserveSample", func() {
				So(scalarLast, ShouldEqual, sampleLast)
			})
		})
	}
}

func TestZScore_ObserveSamples(testingTB *testing.T) {
	Convey("Given a sample batch", testingTB, func() {
		samples := []float64{10, 20, 15, 25, 30}
		batch := NewZScore()
		sequential := NewZScore()

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

func TestZScore_Reset(testingTB *testing.T) {
	Convey("Given reset after warm stream", testingTB, func() {
		surprise := NewZScore()

		for _, sample := range []float64{10, 20, 30, 40} {
			_ = surprise.ObserveSample(sample)
		}

		So(surprise.Reset(), ShouldBeNil)

		got := surprise.ObserveSample(99)

		Convey("It should bootstrap again", func() {
			So(got, ShouldEqual, 0)
			So(surprise.Ready, ShouldBeTrue)
		})
	})
}

func BenchmarkZScore_ObserveSample(b *testing.B) {
	surprise := NewZScore()

	for _, sample := range []float64{10, 20, 15} {
		_ = surprise.ObserveSample(sample)
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = surprise.ObserveSample(11)
	}
}
