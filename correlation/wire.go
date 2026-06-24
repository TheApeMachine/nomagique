package correlation

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

func wireEpochLevel(
	config *datura.Artifact,
	state *datura.Artifact,
	stage string,
) (int64, float64, error) {
	sampleKey := datura.Peek[string](config, "sampleKey")

	if sampleKey == "" {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			stage+": sampleKey required",
			nil,
		))
	}

	pairedKey := datura.Peek[string](config, "pairedKey")

	if pairedKey == "" {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			stage+": pairedKey required",
			nil,
		))
	}

	wireRoot := datura.Peek[string](state, "root")

	if wireRoot == "" {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			stage+": root required",
			nil,
		))
	}

	wireInputs := datura.Peek[[]string](state, "inputs")

	if len(wireInputs) == 0 {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			stage+": inputs required",
			nil,
		))
	}

	var epoch float64
	epochFound := false

	for wireIndex, wireInput := range wireInputs {
		if wireInput != sampleKey {
			continue
		}

		if wireRoot == "features" {
			features := datura.Peek[[]float64](state, wireRoot)

			if wireIndex >= len(features) {
				return 0, 0, errnie.Error(errnie.Err(
					errnie.Validation,
					stage+": feature index out of range",
					nil,
				))
			}

			epoch = features[wireIndex]
		}

		if wireRoot != "features" {
			epoch = datura.Peek[float64](state, wireRoot, wireInput)
		}

		epochFound = true
	}

	if !epochFound {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			stage+": sampleKey not in inputs",
			nil,
		))
	}

	var level float64
	levelFound := false

	for wireIndex, wireInput := range wireInputs {
		if wireInput != pairedKey {
			continue
		}

		if wireRoot == "features" {
			features := datura.Peek[[]float64](state, wireRoot)

			if wireIndex >= len(features) {
				return 0, 0, errnie.Error(errnie.Err(
					errnie.Validation,
					stage+": feature index out of range",
					nil,
				))
			}

			level = features[wireIndex]
		}

		if wireRoot != "features" {
			level = datura.Peek[float64](state, wireRoot, wireInput)
		}

		levelFound = true
	}

	if !levelFound {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			stage+": pairedKey not in inputs",
			nil,
		))
	}

	return int64(epoch), level, nil
}

/*
IntervalWireConfig returns an interval stage config with sample and paired wire keys.
*/
func IntervalWireConfig(name string) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).
		Poke("sample", "sampleKey").
		Poke("paired", "pairedKey")
}

/*
EpochLevelWire stamps epoch and level on inbound wire for interval stages.
*/
func EpochLevelWire(artifact *datura.Artifact, epoch float64, level float64) *datura.Artifact {
	artifact.Poke("wire", "root")
	artifact.Poke([]string{"sample", "paired"}, "inputs")
	artifact.Merge("wire", map[string]any{
		"sample": epoch,
		"paired": level,
	})

	return artifact
}
