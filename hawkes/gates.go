package hawkes

import (
	"math"

	"github.com/theapemachine/nomagique/statistic"
)

/*
FitGates carries series-local saturation and frenzy thresholds derived from fit history.
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
	_, longWindow, err := statistic.RollingWindows(make([]float64, len(spectralRadii)), 0, 0)

	if err != nil || len(spectralRadii) < longWindow || len(asymmetries) < longWindow {
		return FitGates{}, false
	}

	upperRank := 1 - 1/float64(longWindow)
	lowerRank := 1 / float64(longWindow)
	saturationRadius := quantileFromHistory(spectralRadii, upperRank)

	absAsymmetries := make([]float64, len(asymmetries))

	for index, asymmetry := range asymmetries {
		absAsymmetries[index] = math.Abs(asymmetry)
	}

	frenzyAsymmetry := quantileFromHistory(absAsymmetries, lowerRank)

	if saturationRadius <= 0 || frenzyAsymmetry <= 0 {
		return FitGates{}, false
	}

	return FitGates{
		SaturationRadius: saturationRadius,
		FrenzyAsymmetry:  frenzyAsymmetry,
	}, true
}

func quantileFromHistory(history []float64, percentile float64) float64 {
	value, ok := statistic.QuantileOf(percentile, history)

	if !ok {
		return 0
	}

	return value
}
