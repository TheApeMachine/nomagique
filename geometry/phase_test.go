package geometry

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func TestCouplingRead(testingTB *testing.T) {
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
			stage := NewCoupling(couplingConfig("coupling-config"))
			artifact := couplingWire(datura.Acquire("test", datura.APPJSON), testCase.left, testCase.right)
			err := nomagique.RoundTripArtifact(artifact, stage)

			So(err, ShouldBeNil)

			got := datura.Peek[float64](artifact, "output", "value")

			Convey("It should return the expected coupling", func() {
				So(got, ShouldAlmostEqual, testCase.expect, 1e-9)
			})
		})
	}

	Convey("Given empty inbound wire", testingTB, func() {
		stage := NewCoupling(couplingConfig("coupling-config"))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := nomagique.RoundTripArtifact(artifact, stage)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestVelocityRead(testingTB *testing.T) {
	Convey("Given empty inbound wire", testingTB, func() {
		stage := NewVelocity(velocityConfig("velocity-config"))
		artifact := datura.Acquire("test", datura.APPJSON)
		err := nomagique.RoundTripArtifact(artifact, stage)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a first velocity sample", testingTB, func() {
		stage := NewVelocity(velocityConfig("velocity-config"))
		artifact := velocityWire(datura.Acquire("test", datura.APPJSON), 1)
		err := nomagique.RoundTripArtifact(artifact, stage)

		Convey("It should return a span error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given velocity history", testingTB, func() {
		stage := NewVelocity(velocityConfig("velocity-config"))
		artifact := velocityWire(datura.Acquire("test", datura.APPJSON), 1)
		_ = nomagique.RoundTripArtifact(artifact, stage)

		artifact = velocityWire(datura.Acquire("test", datura.APPJSON), 1.5)
		err := nomagique.RoundTripArtifact(artifact, stage)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return the velocity", func() {
			So(got, ShouldAlmostEqual, 0.5, 1e-12)
		})
	})
}

func BenchmarkCouplingRead(testingTB *testing.B) {
	stage := NewCoupling(couplingConfig("coupling-config-bench"))
	artifact := datura.Acquire("test", datura.APPJSON)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		couplingWire(artifact, 1.7, -0.9)
		_ = nomagique.RoundTripArtifact(artifact, stage)
	}
}

func BenchmarkVelocityRead(testingTB *testing.B) {
	stage := NewVelocity(velocityConfig("velocity-config-bench"))
	artifact := velocityWire(datura.Acquire("test", datura.APPJSON), 1)
	_ = nomagique.RoundTripArtifact(artifact, stage)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		velocityWire(artifact, 1.5)
		_ = nomagique.RoundTripArtifact(artifact, stage)
	}
}
