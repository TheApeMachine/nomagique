package geometry

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestCoupling_Observe(testingTB *testing.T) {
	cases := []struct {
		name   string
		left   float64
		right  float64
		expect float64
	}{
		{"co-moving growth", 2, 2, 1},
		{"opposing growth", 2, -2, -1},
		{"below relative floor", 0.001, 0.001, 0},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			stage := NewCoupling(datura.Acquire("coupling-config", datura.APPJSON))
			artifact := datura.Acquire("test", datura.APPJSON).
				Poke(testCase.left, "sample").
				Poke(testCase.right, "paired")
			err := transport.NewFlipFlop(artifact, stage)

			So(err, ShouldBeNil)

			got := datura.Peek[float64](artifact, "output", "value")

			Convey("It should return the expected coupling", func() {
				So(got, ShouldAlmostEqual, testCase.expect, 1e-9)
			})
		})
	}

	Convey("Given empty Observe inputs", testingTB, func() {
		stage := NewCoupling(datura.Acquire("coupling-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should return zero output", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)
		})
	})

	Convey("Given a non-scalar first input", testingTB, func() {
		stage := NewCoupling(datura.Acquire("coupling-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(2, "sample").Poke(2, "paired")
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		before := datura.Peek[float64](artifact, "output", "value")

		fresh := datura.Acquire("test", datura.APPJSON)
		err = transport.NewFlipFlop(fresh, stage)

		So(err, ShouldBeNil)

		Convey("It should leave output unchanged", func() {
			So(datura.Peek[float64](fresh, "output", "value"), ShouldEqual, before)
		})
	})
}

func TestCoupling_Reset(testingTB *testing.T) {
	Convey("Given an observed coupling stage", testingTB, func() {
		stage := NewCoupling(datura.Acquire("coupling-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(2, "sample").
			Poke(2, "paired")
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)
		So(stage.Reset(), ShouldBeNil)

		fresh := datura.Acquire("test", datura.APPJSON)
		err = transport.NewFlipFlop(fresh, stage)

		So(err, ShouldBeNil)

		Convey("It should clear output", func() {
			So(datura.Peek[float64](fresh, "output", "value"), ShouldEqual, 0)
		})
	})
}

func TestVelocity_Observe(testingTB *testing.T) {
	Convey("Given empty Observe inputs", testingTB, func() {
		stage := NewVelocity(datura.Acquire("velocity-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should return zero output", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)
		})
	})

	Convey("Given velocity history", testingTB, func() {
		stage := NewVelocity(datura.Acquire("velocity-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(1, "sample")
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		artifact.Poke(1.5, "sample")
		err = transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return the velocity", func() {
			So(got, ShouldAlmostEqual, 0.5, 1e-12)
		})
	})

	Convey("Given a scalar plus work sample", testingTB, func() {
		stage := NewVelocity(datura.Acquire("velocity-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON)

		artifact.Poke(1, "sample")
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should match a single combined scalar", func() {
			artifact.Poke(1.5, "sample")
			err := transport.NewFlipFlop(artifact, stage)

			So(err, ShouldBeNil)

			withWork := datura.Peek[float64](artifact, "output", "value")

			expect := NewVelocity(datura.Acquire("velocity-config-expect", datura.APPJSON))
			expectArtifact := datura.Acquire("test", datura.APPJSON)

			expectArtifact.Poke(1, "sample")
			err = transport.NewFlipFlop(expectArtifact, expect)

			So(err, ShouldBeNil)

			expectArtifact.Poke(1.5, "sample")
			err = transport.NewFlipFlop(expectArtifact, expect)

			So(err, ShouldBeNil)

			combined := datura.Peek[float64](expectArtifact, "output", "value")

			So(withWork, ShouldEqual, combined)
		})
	})
}

func TestVelocity_ObserveSamples(testingTB *testing.T) {
	Convey("Given mean samples", testingTB, func() {
		stage := NewVelocity(datura.Acquire("velocity-config", datura.APPJSON))
		means := []float64{1, 1.5, 1.25}
		out := make([]float64, len(means))

		stage.ObserveSamples(means, out)

		Convey("It should match sequential Observe", func() {
			expect := NewVelocity(datura.Acquire("velocity-config-expect", datura.APPJSON))
			expectOut := make([]float64, len(means))
			expect.ObserveSamples(means, expectOut)

			So(out, ShouldResemble, expectOut)
		})
	})
}

func TestVelocity_Reset(testingTB *testing.T) {
	Convey("Given an observed velocity stage", testingTB, func() {
		stage := NewVelocity(datura.Acquire("velocity-config", datura.APPJSON))
		artifact := datura.Acquire("test", datura.APPJSON).Poke(1, "sample")
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)
		So(stage.Reset(), ShouldBeNil)

		fresh := datura.Acquire("test", datura.APPJSON)
		err = transport.NewFlipFlop(fresh, stage)

		So(err, ShouldBeNil)

		Convey("It should clear derived state", func() {
			So(stage.ready, ShouldBeFalse)
			So(datura.Peek[float64](fresh, "output", "value"), ShouldEqual, 0)
		})
	})
}

func BenchmarkCoupling_Observe(testingTB *testing.B) {
	stage := NewCoupling(datura.Acquire("coupling-config-bench", datura.APPJSON))
	artifact := datura.Acquire("test", datura.APPJSON)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact.Poke(1.7, "sample").Poke(-0.9, "paired")
		_ = transport.NewFlipFlop(artifact, stage)
	}
}

func BenchmarkVelocity_Observe(testingTB *testing.B) {
	stage := NewVelocity(datura.Acquire("velocity-config-bench", datura.APPJSON))
	artifact := datura.Acquire("test", datura.APPJSON)

	artifact.Poke(1, "sample")
	_ = transport.NewFlipFlop(artifact, stage)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact.Poke(1.5, "sample")
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
