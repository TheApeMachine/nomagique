package hawkes

import (
	"math"
	"sort"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
	"gonum.org/v1/gonum/stat"
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
	_, longWindow, err := statistic.ResolveWindows(make([]float64, len(spectralRadii)), 0, 0)

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
	if len(history) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes gates: quantile requires history",
			nil,
		))
	}

	sorted := append([]float64(nil), history...)
	sort.Float64s(sorted)

	for _, value := range sorted {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"hawkes gates: quantile sample is non-finite",
				nil,
			))
		}
	}

	return stat.Quantile(percentile, stat.LinInterp, sorted, nil), nil
}
