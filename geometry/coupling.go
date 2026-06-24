package geometry

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Coupling measures directional alignment of two growth samples in [-1, +1].
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Coupling struct {
	artifact *datura.Artifact
}

/*
NewCoupling returns a coupling stage wired from config attributes on the artifact.
*/
func NewCoupling(artifact *datura.Artifact) *Coupling {
	return &Coupling{
		artifact: artifact,
	}
}

func (coupling *Coupling) Read(payload []byte) (int, error) {
	state := datura.Acquire("coupling-state", datura.APPJSON)

	if _, err := state.Write(coupling.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"coupling: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"coupling: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"coupling: inputs required",
			nil,
		))
	}

	sampleKey := datura.Peek[string](coupling.artifact, "sampleKey")

	if sampleKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"coupling: sampleKey required",
			nil,
		))
	}

	pairedKey := datura.Peek[string](coupling.artifact, "pairedKey")

	if pairedKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"coupling: pairedKey required",
			nil,
		))
	}

	var leftGrowth float64
	var rightGrowth float64
	leftFound := false
	rightFound := false

	for index, input := range inputs {
		if input == sampleKey {
			if rootKey == "features" {
				features := datura.Peek[[]float64](state, rootKey)

				if index >= len(features) {
					return 0, errnie.Error(errnie.Err(
						errnie.Validation,
						"coupling: feature index out of range",
						nil,
					))
				}

				leftGrowth = features[index]
			}

			if rootKey != "features" {
				leftGrowth = datura.Peek[float64](state, rootKey, input)
			}

			leftFound = true
		}

		if input == pairedKey {
			if rootKey == "features" {
				features := datura.Peek[[]float64](state, rootKey)

				if index >= len(features) {
					return 0, errnie.Error(errnie.Err(
						errnie.Validation,
						"coupling: feature index out of range",
						nil,
					))
				}

				rightGrowth = features[index]
			}

			if rootKey != "features" {
				rightGrowth = datura.Peek[float64](state, rootKey, input)
			}

			rightFound = true
		}
	}

	if !leftFound || !rightFound {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"coupling: require two growth samples",
			ErrEmptyInputs,
		))
	}

	absLeft := math.Abs(leftGrowth)
	absRight := math.Abs(rightGrowth)
	geometricMean := math.Sqrt(absLeft * absRight)
	derived := 0.0

	if geometricMean == 0 {
		coupling.artifact.Poke(derived, "output", "value")
		state.MergeOutput("value", derived)
		state.Poke("output", "root")
		state.Poke([]string{"value"}, "inputs")

		return state.Read(payload)
	}

	denominator := absLeft + absRight

	if denominator == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"coupling: denominator is zero",
			nil,
		))
	}

	relativeFloor := (absLeft * absRight) / denominator

	if geometricMean*geometricMean < relativeFloor {
		derived = 0
	}

	if geometricMean*geometricMean >= relativeFloor {
		derived = (leftGrowth * rightGrowth) / (geometricMean * geometricMean)
	}

	coupling.artifact.Poke(derived, "output", "value")
	state.MergeOutput("value", derived)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.Read(payload)
}

func (coupling *Coupling) Write(payload []byte) (int, error) {
	coupling.artifact.WithPayload(payload)
	return len(payload), nil
}

func (coupling *Coupling) Close() error {
	return nil
}
