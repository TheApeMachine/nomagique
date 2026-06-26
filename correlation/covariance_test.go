package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func covarianceConfig() *datura.Artifact {
	return datura.Acquire("covariance-config", datura.APPJSON)
}

func TestCovarianceRead(testingTB *testing.T) {
	Convey("Given positively coupled streams", testingTB, func() {
		covariance := NewCovariance(covarianceConfig())
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke([]float64{1, 2, 3, 4, 2, 4, 6, 8}, "batch")
		err := nomagique.RoundTripArtifact(artifact, covariance)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return positive covariance", func() {
			So(got, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given empty Observe inputs", testingTB, func() {
		covariance := NewCovariance(covarianceConfig())
		artifact := datura.Acquire("test", datura.APPJSON)
		err := nomagique.RoundTripArtifact(artifact, covariance)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestCovariance_Reset(testingTB *testing.T) {
	Convey("Given an observed covariance stage", testingTB, func() {
		covariance := NewCovariance(covarianceConfig())
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke([]float64{1, 2, 3, 4, 2, 4, 6, 8}, "batch")
		err := nomagique.RoundTripArtifact(artifact, covariance)

		So(err, ShouldBeNil)

		resetArtifact := datura.Acquire("test", datura.APPJSON).Poke(1, "reset")
		err = nomagique.RoundTripArtifact(resetArtifact, covariance)

		So(err, ShouldNotBeNil)

		fresh := datura.Acquire("test", datura.APPJSON)
		err = nomagique.RoundTripArtifact(fresh, covariance)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkCovarianceRead(testingTB *testing.B) {
	covariance := NewCovariance(covarianceConfig())
	artifact := datura.Acquire("test", datura.APPJSON)

	for testingTB.Loop() {
		artifact.Poke([]float64{1, 2, 3, 4, 5, 6, 7, 8, 2, 4, 6, 8, 10, 12, 14, 16}, "batch")
		_ = nomagique.RoundTripArtifact(artifact, covariance)
	}
}
