package correlation

import (
	"testing"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
)

func numberInputs(series ...float64) []core.Number[float64] {
	return nomagique.Numbers(series...)
}

func mustNumbers(testingTB testing.TB, series ...float64) []core.Number[float64] {
	testingTB.Helper()

	return numberInputs(series...)
}

func splitInputs(left, right []float64) []core.Number[float64] {
	inputs := make([]core.Number[float64], 0, len(left)+len(right))
	inputs = append(inputs, numberInputs(left...)...)
	inputs = append(inputs, numberInputs(right...)...)

	return inputs
}
