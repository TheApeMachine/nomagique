package hawkes

import (
	"math"
	"slices"

	"github.com/theapemachine/nomagique/statistic"
	"gonum.org/v1/gonum/stat"
)

/*
FitGateEstimator derives symbol-local gates while reusing quantile storage.
*/
type FitGateEstimator struct {
	window      []float64
	radii       []float64
	asymmetries []float64
}

/*
NewFitGateEstimator returns a reusable Hawkes gate estimator.
*/
func NewFitGateEstimator() *FitGateEstimator {
	return &FitGateEstimator{}
}

/*
Measure derives the same gates as FitGatesFromHistory without transient slices.
*/
func (estimator *FitGateEstimator) Measure(
	spectralRadii, asymmetries []float64,
) (FitGates, bool) {
	estimator.window = slices.Grow(estimator.window[:0], len(spectralRadii))
	estimator.window = estimator.window[:len(spectralRadii)]
	_, longWindow, err := statistic.ResolveWindows(estimator.window, 0, 0)

	if err != nil || len(spectralRadii) < longWindow || len(asymmetries) < longWindow {
		return FitGates{}, false
	}

	upperRank := 1 - 1/float64(longWindow)
	lowerRank := 1 / float64(longWindow)
	saturationRadius, ok := estimator.quantile(
		spectralRadii,
		upperRank,
		false,
		&estimator.radii,
	)

	if !ok {
		return FitGates{}, false
	}

	frenzyAsymmetry, ok := estimator.quantile(
		asymmetries,
		lowerRank,
		true,
		&estimator.asymmetries,
	)

	if !ok {
		return FitGates{}, false
	}

	if saturationRadius <= 0 {
		saturationRadius = criticalBranch
	}

	if frenzyAsymmetry <= 0 {
		frenzyAsymmetry = 1
	}

	return FitGates{
		SaturationRadius: saturationRadius,
		FrenzyAsymmetry:  frenzyAsymmetry,
	}, true
}

func (estimator *FitGateEstimator) quantile(
	history []float64,
	percentile float64,
	absolute bool,
	scratch *[]float64,
) (float64, bool) {
	*scratch = slices.Grow((*scratch)[:0], len(history))

	for _, value := range history {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}

		if absolute {
			value = math.Abs(value)
		}

		*scratch = append(*scratch, value)
	}

	slices.Sort(*scratch)

	return stat.Quantile(percentile, stat.LinInterp, *scratch, nil), true
}
