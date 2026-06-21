package causal

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Contagion peaks cross-node magnitude from the latest table row for regime gating.
The constructor artifact holds config; Write buffers inbound table wire on its payload.
*/
type Contagion struct {
	artifact *datura.Artifact
}

/*
NewContagion returns a contagion stage wired from config attributes on the artifact.
*/
func NewContagion(artifact *datura.Artifact) *Contagion {
	artifact.Inspect("causal", "contagion", "NewContagion()")

	return &Contagion{
		artifact: artifact,
	}
}

func (contagion *Contagion) Write(p []byte) (int, error) {
	contagion.artifact.WithPayload(p)
	return len(p), nil
}

func (contagion *Contagion) Read(p []byte) (int, error) {
	state := datura.Acquire("contagion-state", datura.APPJSON)
	state.Inspect("causal", "contagion", "Read()", "p")

	if _, err := state.Write(contagion.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	peak := contagion.peakFromTable(state)
	state.Merge("paired", peak)
	state.MergeOutput("value", peak)
	return state.Read(p)
}

func (contagion *Contagion) Close() error {
	return nil
}

func (contagion *Contagion) peakFromTable(state *datura.Artifact) float64 {
	rows, ok := tableRows(state)

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
