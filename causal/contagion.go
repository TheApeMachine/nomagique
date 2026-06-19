package causal

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Contagion peaks cross-node magnitude from the latest table row for regime gating.
*/
type Contagion struct {
	artifact *datura.Artifact
}

/*
NewContagion returns a contagion stage that writes paired from table history.
*/
func NewContagion() *Contagion {
	return &Contagion{
		artifact: datura.Acquire("contagion", datura.APPJSON).RetainStageAttributes(),
	}
}

func (contagion *Contagion) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](contagion.artifact, "output") == nil

	contagion.artifact.Clear("sample")
	contagion.artifact.Clear("paired")

	n, err := contagion.artifact.Write(p)

	if bootstrap {
		contagion.artifact.Clear("output")
	}

	return n, err
}

func (contagion *Contagion) Read(p []byte) (int, error) {
	peak := contagion.peakFromTable()

	contagion.artifact.Poke(peak, "paired")
	contagion.artifact.Poke(datura.Map[float64]{"value": peak}, "output")

	return contagion.artifact.Read(p)
}

func (contagion *Contagion) Close() error {
	return nil
}

func (contagion *Contagion) peakFromTable() float64 {
	rows, ok := tableRows(contagion.artifact)

	if !ok {
		return 0
	}

	rowCount := len(rows)

	if rowCount <= 0 {
		return 0
	}

	latestRow := rows[rowCount-1]
	skip := intSlice(datura.Peek[[]float64](contagion.artifact, "config", "contagionSkip"))
	target := int(datura.Peek[float64](contagion.artifact, "config", "target"))
	peak := 0.0

	for nodeIndex, sample := range latestRow {
		if nodeIndex == target || containsIndex(skip, nodeIndex) {
			continue
		}

		magnitude := math.Abs(sample)

		if magnitude > peak {
			peak = magnitude
		}
	}

	return peak
}

func containsIndex(indices []int, nodeIndex int) bool {
	for _, skipIndex := range indices {
		if skipIndex == nodeIndex {
			return true
		}
	}

	return false
}
