package probability

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
CUSUM accumulates sequential change evidence from a sample stream.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type CUSUM struct {
	artifact *datura.Artifact
}

/*
NewCUSUM returns a change-detection stage wired from config attributes on the artifact.
*/
func NewCUSUM(artifact *datura.Artifact) *CUSUM {
	return &CUSUM{
		artifact: artifact,
	}
}

func (cusum *CUSUM) Read(payload []byte) (int, error) {
	state := datura.Acquire("cusum-state", datura.APPJSON)

	if _, err := state.Write(cusum.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"cusum: state write failed",
			err,
		))
	}

	defer state.Release()

	if datura.Peek[float64](state, "reset") != 0 {
		cusum.artifact.Poke(0.0, "output", "target")
		cusum.artifact.Poke(0.0, "output", "positive")
		cusum.artifact.Poke(0.0, "output", "prev")
		cusum.artifact.Poke(0.0, "output", "min")
		cusum.artifact.Poke(0.0, "output", "max")
		cusum.artifact.Poke(0.0, "output", "rate")
		cusum.artifact.Poke(0.0, "output", "ready")
		cusum.artifact.Poke(0.0, "output", "value")
		state.MergeOutput("ready", 0)
		state.MergeOutput("value", 0)
		state.Poke("output", "root")
		state.Poke([]string{"value"}, "inputs")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"cusum: reset",
			nil,
		))
	}

	sampleKey := datura.Peek[string](cusum.artifact, "sampleKey")

	if sampleKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"cusum: sampleKey required",
			nil,
		))
	}

	wireRoot := datura.Peek[string](state, "root")
	
	if wireRoot == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"cusum: root required",
			nil,
		))
	}
	
	wireInputs := datura.Peek[[]string](state, "inputs")
	
	if len(wireInputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"cusum: inputs required",
			nil,
		))
	}
	
	var sample float64
	sampleFound := false
	
	for wireIndex, wireInput := range wireInputs {
		if wireInput != sampleKey {
			continue
		}
	
		if wireRoot == "features" {
			features := datura.Peek[[]float64](state, wireRoot)
	
			if wireIndex >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"cusum: feature index out of range",
					nil,
				))
			}
	
			sample = features[wireIndex]
		}
	
		if wireRoot != "features" {
			sample = datura.Peek[float64](state, wireRoot, wireInput)
		}
	
		sampleFound = true
	}
	
	if !sampleFound {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"cusum: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"cusum: sample is non-finite",
			nil,
		))
	}

	cusumState := CUSUMState{
		Target:   datura.Peek[float64](cusum.artifact, "output", "target"),
		Positive: datura.Peek[float64](cusum.artifact, "output", "positive"),
		Prev:     datura.Peek[float64](cusum.artifact, "output", "prev"),
		Min:      datura.Peek[float64](cusum.artifact, "output", "min"),
		Max:      datura.Peek[float64](cusum.artifact, "output", "max"),
		Rate:     datura.Peek[float64](cusum.artifact, "output", "rate"),
		Ready:    datura.Peek[float64](cusum.artifact, "output", "ready") != 0,
	}

	value := ObserveCUSUM(&cusumState, sample)

	ready := 0.0

	if cusumState.Ready {
		ready = 1
	}

	cusum.artifact.Poke(cusumState.Target, "output", "target")
	cusum.artifact.Poke(cusumState.Positive, "output", "positive")
	cusum.artifact.Poke(cusumState.Prev, "output", "prev")
	cusum.artifact.Poke(cusumState.Min, "output", "min")
	cusum.artifact.Poke(cusumState.Max, "output", "max")
	cusum.artifact.Poke(cusumState.Rate, "output", "rate")
	cusum.artifact.Poke(ready, "output", "ready")
	cusum.artifact.Poke(value, "output", "value")
	state.MergeOutput("value", value)
	state.MergeOutput("ready", ready)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.Read(payload)
}

func (cusum *CUSUM) Write(payload []byte) (int, error) {
	cusum.artifact.WithPayload(payload)
	return len(payload), nil
}

func (cusum *CUSUM) Close() error {
	return nil
}

/*
CUSUMState accumulates one-sided change evidence from a sample stream.
*/
type CUSUMState struct {
	Target   float64
	Positive float64
	Prev     float64
	Min      float64
	Max      float64
	Rate     float64
	Ready    bool
}

/*
Observe ingests one sample and returns cumulative change evidence.
*/
func (state *CUSUMState) Observe(sample float64) float64 {
	return ObserveCUSUM(state, sample)
}

/*
ObserveSamples writes one evidence value per sample into out.
*/
func (state *CUSUMState) ObserveSamples(samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = ObserveCUSUM(state, sample)
	}
}

/*
Reset clears derived state.
*/
func (state *CUSUMState) Reset() {
	state.Target = 0
	state.Positive = 0
	state.Prev = 0
	state.Min = 0
	state.Max = 0
	state.Rate = 0
	state.Ready = false
}

/*
ObserveCUSUM ingests one sample and returns one-sided cumulative evidence.
*/
func ObserveCUSUM(state *CUSUMState, sample float64) float64 {
	if !state.Ready {
		state.Target = sample
		state.Prev = sample
		state.Min = sample
		state.Max = sample
		state.Positive = 0
		state.Ready = true
		return 0
	}

	return observeCUSUMReady(state, sample)
}

/*
observeCUSUMReady runs the hot CUSUM path; state must already be Ready.
*/
func observeCUSUMReady(state *CUSUMState, sample float64) float64 {
	state.Min = math.Min(state.Min, sample)
	state.Max = math.Max(state.Max, sample)

	span := state.Max - state.Min

	if span == 0 {
		state.Prev = sample
		return state.Positive
	}

	state.Rate = math.Abs(sample-state.Prev) / span
	drift := state.Rate * span / 2
	excess := sample - state.Target - drift

	if excess > 0 {
		state.Positive += excess
	}

	if excess < 0 {
		state.Positive = 0
	}

	state.Target += state.Rate * (sample - state.Target)
	state.Prev = sample

	return state.Positive
}
