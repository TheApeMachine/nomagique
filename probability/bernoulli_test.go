package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func TestBernoulli(testingTB *testing.T) {
	Convey("Given Bernoulli constructor", testingTB, func() {
		posterior := NewBernoulli(bernoulliConfig("bernoulli-config"))

		Convey("It should return a usable dynamic", func() {
			So(posterior, ShouldNotBeNil)
		})
	})
}

func TestBernoulliRead(testingTB *testing.T) {
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
			posterior := NewBernoulli(bernoulliConfig("bernoulli-config"))
			artifact := datura.Acquire("test", datura.APPJSON)

			for _, sample := range testCase.inputs {
				scalarWire(artifact, "sample", sample)
				err := nomagique.RoundTripArtifact(artifact, posterior)

				So(err, ShouldBeNil)
			}

			got := datura.Peek[float64](artifact, "output", "value")

			Convey("It should return the expected posterior mean", func() {
				So(testCase.expect(got), ShouldBeTrue)
			})
		})
	}

	Convey("Given empty Observe inputs", testingTB, func() {
		posterior := NewBernoulli(bernoulliConfig("bernoulli-config"))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := nomagique.RoundTripArtifact(artifact, posterior)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a predicted and actual pair", testingTB, func() {
		posterior := NewBernoulli(bernoulliPairConfig("bernoulli-config"))
		artifact := pairWire(datura.Acquire("test", datura.APPJSON), "sample", "paired", 10, 15)
		err := nomagique.RoundTripArtifact(artifact, posterior)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should raise hit probability", func() {
			So(got, ShouldBeGreaterThan, 0.5)
		})
	})

	Convey("Given repeated equal outcomes", testingTB, func() {
		posterior := NewBernoulli(bernoulliConfig("bernoulli-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		for _, sample := range []float64{1, 1} {
			scalarWire(artifact, "sample", sample)
			err := nomagique.RoundTripArtifact(artifact, posterior)

			So(err, ShouldBeNil)
		}

		Convey("It should update the Beta posterior once per observation", func() {
			So(datura.Peek[float64](posterior.artifact, "output", "alpha"), ShouldEqual, 3)
			So(datura.Peek[float64](posterior.artifact, "output", "beta"), ShouldEqual, 1)
			So(datura.Peek[float64](posterior.artifact, "output", "count"), ShouldEqual, 2)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0.75)
		})
	})

	Convey("Given repeated equal prediction errors", testingTB, func() {
		posterior := NewBernoulli(bernoulliPairConfig("bernoulli-config"))
		artifact := datura.Acquire("test", datura.APPJSON)

		for range 2 {
			pairWire(artifact, "sample", "paired", 10, 10)
			err := nomagique.RoundTripArtifact(artifact, posterior)

			So(err, ShouldBeNil)
		}

		Convey("It should update paired observations without span errors", func() {
			So(datura.Peek[float64](posterior.artifact, "output", "alpha"), ShouldEqual, 3)
			So(datura.Peek[float64](posterior.artifact, "output", "beta"), ShouldEqual, 1)
			So(datura.Peek[float64](posterior.artifact, "output", "count"), ShouldEqual, 2)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0.75)
		})
	})

	Convey("Given an invalid outcome", testingTB, func() {
		posterior := NewBernoulli(bernoulliConfig("bernoulli-config"))
		artifact := scalarWire(datura.Acquire("test", datura.APPJSON), "sample", 2)
		err := nomagique.RoundTripArtifact(artifact, posterior)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestBernoulliReset(testingTB *testing.T) {
	Convey("Given an observed posterior", testingTB, func() {
		posterior := NewBernoulli(bernoulliConfig("bernoulli-config"))
		artifact := scalarWire(datura.Acquire("test", datura.APPJSON), "sample", 1)

		err := nomagique.RoundTripArtifact(artifact, posterior)

		So(err, ShouldBeNil)

		resetArtifact := datura.Acquire("test", datura.APPJSON).Poke(1, "reset")
		err = nomagique.RoundTripArtifact(resetArtifact, posterior)

		So(err, ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(datura.Peek[float64](resetArtifact, "output", "count"), ShouldEqual, 0)
			So(datura.Peek[float64](resetArtifact, "output", "value"), ShouldEqual, 0)
		})
	})
}

func BenchmarkBernoulliRead(testingTB *testing.B) {
	posterior := NewBernoulli(bernoulliPairConfig("bernoulli-config-bench"))
	artifact := datura.Acquire("test", datura.APPJSON)

	scalarWire(artifact, "sample", 1)
	_ = nomagique.RoundTripArtifact(artifact, posterior)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		pairWire(artifact, "sample", "paired", 10, 11)
		_ = nomagique.RoundTripArtifact(artifact, posterior)
	}
}
