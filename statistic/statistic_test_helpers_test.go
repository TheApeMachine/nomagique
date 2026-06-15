package statistic

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
