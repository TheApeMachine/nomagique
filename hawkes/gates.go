package hawkes

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/statistic"
	"gonum.org/v1/gonum/stat"
)

const minFitGateHistory = 4

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
	if len(spectralRadii) < minFitGateHistory || len(asymmetries) < minFitGateHistory {
		return FitGates{}, false
	}

	saturationRadius := quantileFromHistory(spectralRadii, 0.9)

	absAsymmetries := make([]float64, len(asymmetries))

	for index, asymmetry := range asymmetries {
		absAsymmetries[index] = math.Abs(asymmetry)
	}

	frenzyAsymmetry := quantileFromHistory(absAsymmetries, 0.25)

	if saturationRadius <= 0 || frenzyAsymmetry <= 0 {
		return FitGates{}, false
	}

	return FitGates{
		SaturationRadius: saturationRadius,
		FrenzyAsymmetry:  frenzyAsymmetry,
	}, true
}

func quantileFromHistory(history []float64, percentile float64) float64 {
	artifact := datura.Acquire("hawkes-quantile", datura.APPJSON).Poke(history, "history")
	quantile := statistic.NewQuantile(percentile, stat.LinInterp)
	_ = transport.NewFlipFlop(artifact, quantile)

	return datura.Peek[float64](artifact, "output", "value")
}
