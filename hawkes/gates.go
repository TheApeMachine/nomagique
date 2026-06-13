package hawkes

import (
	"math"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/statistic"
	"gonum.org/v1/gonum/stat"
)

const minFitGateHistory = 4

/*
FitGates carries symbol-local saturation and frenzy thresholds derived from fit history.
*/
type FitGates struct {
	SaturationRadius float64
	FrenzyAsymmetry  float64
}

/*
Ready reports whether both gates were derived from sufficient history.
*/
func (gates FitGates) Ready() bool {
	return gates.SaturationRadius > 0 && gates.FrenzyAsymmetry > 0
}

/*
FitGatesFromHistory derives saturation and frenzy gates from observed fit statistics.
*/
func FitGatesFromHistory(spectralRadii, asymmetries []float64) (FitGates, bool) {
	if len(spectralRadii) < minFitGateHistory || len(asymmetries) < minFitGateHistory {
		return FitGates{}, false
	}

	saturationRadius := float64(
		statistic.NewQuantile(0.9, stat.LinInterp, nil).Observe(nomagique.Numbers(spectralRadii...)...),
	)
	absAsymmetries := make([]float64, len(asymmetries))

	for index, asymmetry := range asymmetries {
		absAsymmetries[index] = math.Abs(asymmetry)
	}

	frenzyAsymmetry := float64(
		statistic.NewQuantile(0.25, stat.LinInterp, nil).Observe(nomagique.Numbers(absAsymmetries...)...),
	)

	if saturationRadius <= 0 || frenzyAsymmetry <= 0 {
		return FitGates{}, false
	}

	return FitGates{
		SaturationRadius: saturationRadius,
		FrenzyAsymmetry:  frenzyAsymmetry,
	}, true
}
