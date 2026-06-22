package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestBernoulli(testingTB *testing.T) {
	Convey("Given Bernoulli constructor", testingTB, func() {
		posterior := NewBernoulli(datura.Acquire("bernoulli-config", datura.APPJSON))

		Convey("It should return a usable dynamic", func() {
			So(posterior, ShouldNotBeNil)
		})
	})
}

func TestPosterior_Observe(testingTB *testing.T) {
	cases := []struct {
		name   string
		inputs []float64
		expect func(float64) bool
	}{
		{
			name:   "unit success",
			inputs: []float64{1},
			expect: func(value float64) bool { return value > 0.5 },
		},
		{
			name:   "unit failure",
			inputs: []float64{0},
			expect: func(value float64) bool { return value < 0.5 },
		},
		{
			name:   "partial outcome",
			inputs: []float64{0.75},
			expect: func(value float64) bool { return value > 0.5 && value < 1 },
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			posterior := NewBernoulli(datura.Acquire("bernoulli-config", datura.APPJSON))
			artifact := datura.Acquire("test", datura.APPJSON)

			for _, sample := range testCase.inputs {
				artifact.Poke(sample, "sample")
				err := transport.NewFlipFlop(artifact, posterior)

				So(err, ShouldBeNil)
			}

			got := datura.Peek[float64](artifact, "output", "value")

			Convey("It should return the expected posterior mean", func() {
				So(testCase.expect(got), ShouldBeTrue)
			})
		})
	}

	Convey("Given empty Observe inputs", testingTB, func() {
		posterior := NewBernoulli(datura.Acquire("bernoulli-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, posterior)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a predicted and actual pair", testingTB, func() {
		posterior := NewBernoulli(datura.Acquire("bernoulli-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(10, "sample").
			Poke(15, "paired")
		err := transport.NewFlipFlop(artifact, posterior)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should raise hit probability", func() {
			So(got, ShouldBeGreaterThan, 0.5)
		})
	})

	Convey("Given an invalid outcome", testingTB, func() {
		posterior := NewBernoulli(datura.Acquire("bernoulli-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(2, "sample")
		err := transport.NewFlipFlop(artifact, posterior)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestPosterior_Reset(testingTB *testing.T) {
	Convey("Given an observed posterior", testingTB, func() {
		posterior := NewBernoulli(datura.Acquire("bernoulli-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(1, "sample")

		err := transport.NewFlipFlop(artifact, posterior)

		So(err, ShouldBeNil)

		resetArtifact := datura.Acquire("test", datura.APPJSON).Poke(1, "reset")
		err = transport.NewFlipFlop(resetArtifact, posterior)

		So(err, ShouldNotBeNil)

		Convey("It should clear derived state", func() {
			So(datura.Peek[float64](posterior.artifact, "output", "ready"), ShouldEqual, 0)
			So(datura.Peek[float64](posterior.artifact, "output", "value"), ShouldEqual, 0)
		})
	})
}

func BenchmarkBernoulli_Observe(testingTB *testing.B) {
	posterior := NewBernoulli(datura.Acquire("bernoulli-config-bench", datura.APPJSON))
	artifact := datura.Acquire("test", datura.APPJSON)

	artifact.Poke(1, "sample")
	_ = transport.NewFlipFlop(artifact, posterior)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact.Poke(10, "sample").Poke(11, "paired")
		_ = transport.NewFlipFlop(artifact, posterior)
	}
}
