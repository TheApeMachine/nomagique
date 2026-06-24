package hawkes

import (
	"math"

	"github.com/theapemachine/errnie"
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
	_, longWindow, err := statistic.NewRollingWindow(0, 0).Resolve(make([]float64, len(spectralRadii)))

	if err != nil || len(spectralRadii) < longWindow || len(asymmetries) < longWindow {
		return FitGates{}, false
	}

	upperRank := 1 - 1/float64(longWindow)
	lowerRank := 1 / float64(longWindow)
	saturationRadius, err := quantileFromHistory(spectralRadii, upperRank)

	if err != nil {
		return FitGates{}, false
	}

	absAsymmetries := make([]float64, len(asymmetries))

	for index, asymmetry := range asymmetries {
		absAsymmetries[index] = math.Abs(asymmetry)
	}

	frenzyAsymmetry, err := quantileFromHistory(absAsymmetries, lowerRank)

	if err != nil {
		return FitGates{}, false
	}

	if saturationRadius <= 0 || frenzyAsymmetry <= 0 {
		return FitGates{}, false
	}

	return FitGates{
		SaturationRadius: saturationRadius,
		FrenzyAsymmetry:  frenzyAsymmetry,
	}, true
}

func quantileFromHistory(history []float64, percentile float64) (float64, error) {
	value, err := statistic.QuantileOf(percentile, history)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes gates: quantile failed",
			err,
		))
	}

	return value, nil
}
