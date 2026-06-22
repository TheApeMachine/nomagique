package learning

import (
	"github.com/bytedance/sonic"
	"github.com/theapemachine/datura"
)

func wireRole(state *datura.Artifact) string {
	role := datura.Peek[string](state, "channel")

	if role == "" {
		role, _ = state.Role()
	}

	return role
}

func configString(config, state *datura.Artifact, key string) string {
	role := wireRole(state)

	if role != "" {
		scoped := datura.Peek[string](config, role, key)

		if scoped != "" {
			return scoped
		}
	}

	return datura.Peek[string](config, key)
}

func configFloat64(config, state *datura.Artifact, key string) float64 {
	role := wireRole(state)

	if role != "" && datura.KeyPresent(config, role, key) {
		return datura.Peek[float64](config, role, key)
	}

	if datura.KeyPresent(config, key) {
		return datura.Peek[float64](config, key)
	}

	return 0
}

func configStringSlice(config, state *datura.Artifact, key string) []string {
	role := wireRole(state)

	if role != "" {
		scoped := datura.Peek[[]string](config, role, key)

		if len(scoped) > 0 {
			return scoped
		}
	}

	return datura.Peek[[]string](config, key)
}

func weightStateFromArtifact(artifact *datura.Artifact) WeightState {
	return WeightState{
		Trust: datura.Peek[float64](artifact, "output", "trust"),
		Prev:  datura.Peek[float64](artifact, "output", "prev"),
		Min:   datura.Peek[float64](artifact, "output", "min"),
		Max:   datura.Peek[float64](artifact, "output", "max"),
		Rate:  datura.Peek[float64](artifact, "output", "rate"),
		Ready: datura.Peek[float64](artifact, "output", "ready") != 0,
	}
}

func pokeWeightState(artifact *datura.Artifact, state *WeightState, value float64) {
	ready := 0.0

	if state.Ready {
		ready = 1
	}

	artifact.Poke(state.Trust, "output", "trust")
	artifact.Poke(state.Prev, "output", "prev")
	artifact.Poke(state.Min, "output", "min")
	artifact.Poke(state.Max, "output", "max")
	artifact.Poke(state.Rate, "output", "rate")
	artifact.Poke(ready, "output", "ready")
	artifact.Poke(value, "output", "value")
}

func sampleRatioStateFromArtifact(artifact *datura.Artifact) SampleRatioState {
	return SampleRatioState{
		Prev:      datura.Peek[float64](artifact, "output", "prev"),
		Min:       datura.Peek[float64](artifact, "output", "min"),
		Max:       datura.Peek[float64](artifact, "output", "max"),
		PeakRatio: datura.Peek[float64](artifact, "output", "peakRatio"),
		Ready:     datura.Peek[float64](artifact, "output", "ready") != 0,
	}
}

func pokeSampleRatioState(artifact *datura.Artifact, state *SampleRatioState, value float64) {
	ready := 0.0

	if state.Ready {
		ready = 1
	}

	artifact.Poke(state.Prev, "output", "prev")
	artifact.Poke(state.Min, "output", "min")
	artifact.Poke(state.Max, "output", "max")
	artifact.Poke(state.PeakRatio, "output", "peakRatio")
	artifact.Poke(ready, "output", "ready")
	artifact.Poke(value, "output", "value")
}

func forecastStateFromArtifact(artifact *datura.Artifact) ForecastState {
	return ForecastState{
		Scale: datura.Peek[float64](artifact, "output", "scale"),
		Weight: WeightState{
			Trust: datura.Peek[float64](artifact, "output", "trust"),
			Prev:  datura.Peek[float64](artifact, "output", "prev"),
			Min:   datura.Peek[float64](artifact, "output", "min"),
			Max:   datura.Peek[float64](artifact, "output", "max"),
			Rate:  datura.Peek[float64](artifact, "output", "rate"),
			Ready: datura.Peek[float64](artifact, "output", "weightReady") != 0,
		},
		Ready: datura.Peek[float64](artifact, "output", "ready") != 0,
	}
}

func pokeForecastState(artifact *datura.Artifact, state *ForecastState, value float64) {
	ready := 0.0

	if state.Ready {
		ready = 1
	}

	weightReady := 0.0

	if state.Weight.Ready {
		weightReady = 1
	}

	artifact.Poke(state.Scale, "output", "scale")
	artifact.Poke(state.Weight.Trust, "output", "trust")
	artifact.Poke(state.Weight.Prev, "output", "prev")
	artifact.Poke(state.Weight.Min, "output", "min")
	artifact.Poke(state.Weight.Max, "output", "max")
	artifact.Poke(state.Weight.Rate, "output", "rate")
	artifact.Poke(weightReady, "output", "weightReady")
	artifact.Poke(ready, "output", "ready")
	artifact.Poke(value, "output", "value")
}

func attributeKeyPresent(artifact *datura.Artifact, key string) bool {
	rawAttributes, err := artifact.Attributes()

	if err == nil && len(rawAttributes) > 0 {
		node, getErr := sonic.Get(rawAttributes, key)

		if getErr == nil && node.Exists() {
			return true
		}
	}

	payload, err := artifact.DecryptPayloadError()

	if err != nil || len(payload) == 0 {
		return false
	}

	node, getErr := sonic.Get(payload, key)

	return getErr == nil && node.Exists()
}
