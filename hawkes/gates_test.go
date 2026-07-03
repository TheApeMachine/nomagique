package hawkes

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFitGatesFromHistory(testingTB *testing.T) {
	Convey("Given enough spectral-radius and asymmetry history", testingTB, func() {
		spectralRadii := []float64{0.45, 0.48, 0.44, 0.46, 0.47, 0.45, 0.48, 0.46, 0.44, 0.47, 0.45, 0.46, 0.48, 0.44}
		asymmetries := []float64{0.04, 0.05, 0.06, 0.05, 0.04, 0.05, 0.06, 0.05, 0.04, 0.05, 0.06, 0.04, 0.05, 0.05}

		gates, gatesReady := FitGatesFromHistory(spectralRadii, asymmetries)

		Convey("It should derive symbol-local gates", func() {
			So(gatesReady, ShouldBeTrue)
			So(gates.SaturationRadius, ShouldBeGreaterThan, 0)
			So(gates.FrenzyAsymmetry, ShouldBeGreaterThan, 0)
			So(gates.SaturationRadius, ShouldBeGreaterThan, gates.FrenzyAsymmetry)
		})
	})

	Convey("Given enough no-excitation history", testingTB, func() {
		spectralRadii := make([]float64, 14)
		asymmetries := make([]float64, 14)

		gates, gatesReady := FitGatesFromHistory(spectralRadii, asymmetries)

		Convey("It should use physical bounds as gates", func() {
			So(gatesReady, ShouldBeTrue)
			So(gates.SaturationRadius, ShouldEqual, criticalBranch)
			So(gates.FrenzyAsymmetry, ShouldEqual, 1)
		})
	})
}
