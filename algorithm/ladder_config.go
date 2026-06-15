package algorithm

import (
	"math"

	"github.com/theapemachine/nomagique/causal"
)

func deriveLadderBandwidth(streams [][]float64, treatmentNode int) float64 {
	rows, ok := zipNodeRows(streams)

	if !ok || treatmentNode < 0 || treatmentNode >= len(rows[0]) {
		return 0
	}

	values := make([]float64, len(rows))

	for index := range rows {
		values[index] = rows[index][treatmentNode]
	}

	if len(values) < 12 {
		return 0
	}

	mean := 0.0

	for _, value := range values {
		mean += value
	}

	mean /= float64(len(values))

	variance := 0.0

	for _, value := range values {
		delta := value - mean
		variance += delta * delta
	}

	if len(values) > 1 {
		variance /= float64(len(values) - 1)
	}

	if variance <= 0 {
		return 0
	}

	return 1.06 * math.Sqrt(variance) * math.Pow(float64(len(values)), -0.2)
}

func deriveConfoundFraction(streams [][]float64, treatmentNode int) float64 {
	rows, ok := zipNodeRows(streams)

	if !ok || treatmentNode < 0 || treatmentNode >= len(rows[0]) {
		return 0
	}

	values := make([]float64, len(rows))

	for index := range rows {
		values[index] = rows[index][treatmentNode]
	}

	mean := 0.0

	for _, value := range values {
		mean += value
	}

	mean /= float64(len(values))

	if mean <= 0 {
		return 0
	}

	variance := 0.0

	for _, value := range values {
		delta := value - mean
		variance += delta * delta
	}

	if len(values) > 1 {
		variance /= float64(len(values) - 1)
	}

	if variance <= 0 {
		return 0
	}

	return math.Min(0.5, math.Sqrt(variance)/mean)
}

func applyDerivedLadderConfig(config causal.LadderConfig, streams [][]float64) causal.LadderConfig {
	if config.KernelBandwidth <= 0 {
		config.KernelBandwidth = deriveLadderBandwidth(streams, config.TreatmentNormal)
	}

	if config.ConfoundFraction <= 0 {
		config.ConfoundFraction = deriveConfoundFraction(streams, config.TreatmentNormal)
	}

	return config
}
