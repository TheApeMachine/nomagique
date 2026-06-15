package vector

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/tests"
)

func TestNewInputSlot(testingTB *testing.T) {
	Convey("Given a valid channel", testingTB, func() {
		extractor, err := newPairExtractor()
		So(err, ShouldBeNil)

		leftSlot, err := NewInputSlot[float64](extractor, testLeftChannel)

		Convey("It should return a usable slot", func() {
			So(err, ShouldBeNil)
			So(leftSlot, ShouldNotBeNil)
		})
	})

	errorCases := []struct {
		name  string
		setup func() (*FeatureExtractor, int)
	}{
		{
			name: "nil extractor",
			setup: func() (*FeatureExtractor, int) {
				return nil, 0
			},
		},
		{
			name: "out of range channel",
			setup: func() (*FeatureExtractor, int) {
				extractor, err := newPairExtractor()
				So(err, ShouldBeNil)

				return extractor, 2
			},
		},
	}

	for _, testCase := range errorCases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			extractor, channel := testCase.setup()
			_, err := NewInputSlot[float64](extractor, channel)

			Convey("It should return an error", func() {
				So(err, ShouldNotBeNil)
			})
		})
	}
}

func TestInputSlot_Observe(testingTB *testing.T) {
	cases := []struct {
		name         string
		sample       float64
		work         float64
		useWork      bool
		expectStored float64
	}{
		{"positive sample", 100, 0, false, 100},
		{"negative sample", -4, 0, false, -4},
		{"zero sample", 0, 0, false, 0},
		{"scalar plus work", 5, 3, true, 8},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			extractor, err := newPairExtractor()
			So(err, ShouldBeNil)

			leftSlot, err := NewInputSlot[float64](extractor, testLeftChannel)
			So(err, ShouldBeNil)

			var got core.Scalar[float64]

			if testCase.useWork {
				got = leftSlot.Observe(
					core.Scalar[float64](testCase.sample),
					core.Scalar[float64](testCase.work),
				)
			}

			if !testCase.useWork {
				got = leftSlot.Observe(core.Scalar[float64](testCase.sample))
			}

			stored, inputErr := extractor.Input(testLeftChannel)

			Convey("It should store and echo the sample", func() {
				So(float64(got), ShouldEqual, testCase.expectStored)
				So(inputErr, ShouldBeNil)
				So(stored, ShouldEqual, testCase.expectStored)
			})
		})
	}

	Convey("Given empty Observe inputs", testingTB, func() {
		leftSlot, err := NewInputSlot[float64](mustPairExtractor(testingTB), testLeftChannel)
		So(err, ShouldBeNil)

		Convey("It should return zero output", func() {
			So(leftSlot.Observe(), ShouldEqual, core.Scalar[float64](0))
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		extractor, err := newPairExtractor()
		So(err, ShouldBeNil)

		leftSlot, err := NewInputSlot[float64](extractor, testLeftChannel)
		So(err, ShouldBeNil)

		_ = leftSlot.Observe(core.Scalar[float64](10))
		stage := &tests.PipelineStage[float64]{Result: core.Scalar[float64](99)}

		Convey("It should leave output unchanged", func() {
			So(leftSlot.Observe(stage), ShouldEqual, core.Scalar[float64](10))

			stored, inputErr := extractor.Input(testLeftChannel)
			So(inputErr, ShouldBeNil)
			So(stored, ShouldEqual, 10)
		})
	})
}

func TestInputSlot_Reset(testingTB *testing.T) {
	Convey("Given an observed slot", testingTB, func() {
		extractor, err := newPairExtractor()
		So(err, ShouldBeNil)

		leftSlot, err := NewInputSlot[float64](extractor, testLeftChannel)
		So(err, ShouldBeNil)

		_ = leftSlot.Observe(core.Scalar[float64](10))

		Convey("When reset", func() {
			So(leftSlot.Reset(), ShouldBeNil)

			Convey("It should succeed without clearing extractor state", func() {
				stored, inputErr := extractor.Input(testLeftChannel)
				So(inputErr, ShouldBeNil)
				So(stored, ShouldEqual, 10)
			})
		})
	})
}

func BenchmarkInputSlot_Observe(b *testing.B) {
	leftSlot, err := NewInputSlot[float64](mustPairExtractor(b), testLeftChannel)

	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = leftSlot.Observe(core.Scalar[float64](100))
	}
}

func mustPairExtractor(testingTB testing.TB) *FeatureExtractor {
	testingTB.Helper()

	extractor, err := newPairExtractor()

	if err != nil {
		testingTB.Fatal(err)
	}

	return extractor
}
