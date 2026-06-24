package learning

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func rlsConfig(name string, dimension int, initialVariance float64) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).
		WithAttribute("dimension", float64(dimension)).
		WithAttribute("initialVariance", initialVariance)
}

func TestRLS(testingTB *testing.T) {
	Convey("Given NewRLS", testingTB, func() {
		stage := NewRLS(rlsConfig("rls-config", 2, 1000))

		Convey("It should return a usable stage", func() {
			So(stage, ShouldNotBeNil)
		})
	})
}

func TestRLSRead(testingTB *testing.T) {
	Convey("Given a non-positive dimension", testingTB, func() {
		stage := NewRLS(rlsConfig("rls-config", 0, 1000))
		artifact := datura.Acquire("test", datura.APPJSON).Poke([]float64{1, 2}, "batch")
		err := transport.NewFlipFlop(artifact, stage)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given feature and target scalars", testingTB, func() {
		stage := NewRLS(rlsConfig("rls-config", 1, 1000))
		artifact := datura.Acquire("test", datura.APPJSON).Poke([]float64{2, 4}, "batch")
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should derive a finite prediction", func() {
			So(got, ShouldBeGreaterThan, 0)
			So(len(datura.Peek[[]float64](stage.stateStore, "output", "beta")), ShouldEqual, 2)
			So(len(datura.Peek[[]float64](stage.stateStore, "output", "covarianceDiagonal")), ShouldEqual, 2)
		})
	})

	Convey("Given a simple linear relation", testingTB, func() {
		stage := NewRLS(rlsConfig("rls-config-linear", 1, 1000))
		artifact := datura.Acquire("test", datura.APPJSON)

		for step := 0; step < 32; step++ {
			feature := float64(step) / 32
			target := 2*feature + 1
			artifact.Poke([]float64{feature, target}, "batch")
			err := transport.NewFlipFlop(artifact, stage)

			So(err, ShouldBeNil)
		}

		artifact.Poke([]float64{0.5, 2}, "batch")
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		forecast := datura.Peek[float64](artifact, "output", "value")

		Convey("It should learn the mapping", func() {
			So(err, ShouldBeNil)
			So(forecast, ShouldAlmostEqual, 2, 0.25)
		})
	})

	Convey("Given a forgetting factor", testingTB, func() {
		config := rlsConfig("rls-config-forgetting", 1, 1000).WithAttribute("forgettingFactor", 0.5)
		stage := NewRLS(config)
		artifact := datura.Acquire("test", datura.APPJSON)

		for step := 0; step < 16; step++ {
			artifact.Poke([]float64{1, 1}, "batch")
			err := transport.NewFlipFlop(artifact, stage)

			So(err, ShouldBeNil)
		}

		for step := 0; step < 16; step++ {
			artifact.Poke([]float64{1, 5}, "batch")
			err := transport.NewFlipFlop(artifact, stage)

			So(err, ShouldBeNil)
		}

		forecast := datura.Peek[float64](artifact, "output", "value")

		Convey("It should adapt faster to the new target", func() {
			So(forecast, ShouldBeGreaterThan, 2.5)
		})
	})

	Convey("Given repeated collinear updates with aggressive forgetting", testingTB, func() {
		config := rlsConfig("rls-config-collinear", 13, 1000).WithAttribute("forgettingFactor", 0.01)
		stage := NewRLS(config)
		artifact := datura.Acquire("test", datura.APPJSON)
		features := make([]float64, 13)

		for index := range features {
			features[index] = 0.42
		}

		for step := 0; step < 4096; step++ {
			target := 0.001 * float64(step%3-1)
			batch := append(append([]float64(nil), features...), target)
			artifact.Poke(batch, "batch")
			err := transport.NewFlipFlop(artifact, stage)

			So(err, ShouldBeNil)
		}

		forecast := datura.Peek[float64](artifact, "output", "value")

		Convey("It should stay numerically stable after repair", func() {
			So(math.IsNaN(forecast), ShouldBeFalse)
			So(math.IsInf(forecast, 0), ShouldBeFalse)
		})
	})

	Convey("Given persisted coefficients across reads", testingTB, func() {
		stage := NewRLS(rlsConfig("rls-config-persist", 1, 1000))
		artifact := datura.Acquire("test", datura.APPJSON)

		for step := 0; step < 8; step++ {
			feature := float64(step) / 8
			artifact.Poke([]float64{feature, 2*feature + 1}, "batch")
			err := transport.NewFlipFlop(artifact, stage)

			So(err, ShouldBeNil)
		}

		beta := datura.Peek[[]float64](stage.stateStore, "output", "beta")

		Convey("It should retain non-zero coefficients on the state artifact", func() {
			So(len(beta), ShouldEqual, 2)
			So(beta[0], ShouldNotEqual, 0)
		})
	})
}

func BenchmarkRLSRead(b *testing.B) {
	stage := NewRLS(rlsConfig("rls-config", 13, 1000).Poke(0.01, "forgettingFactor"))
	artifact := datura.Acquire("test", datura.APPJSON)
	features := make([]float64, 13)

	for index := range features {
		features[index] = 0.42
	}

	b.ReportAllocs()

	for b.Loop() {
		target := 0.001 * float64(b.N%3-1)
		batch := append(append([]float64(nil), features...), target)
		artifact.Poke(batch, "batch")
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
