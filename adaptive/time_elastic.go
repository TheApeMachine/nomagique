package adaptive

import (
	"math"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
TimeElastic tracks a time-decayed baseline and returns sample/baseline ratios.
The constructor artifact holds config; Write buffers inbound payload.
*/
type TimeElastic struct {
	artifact *datura.Artifact
	baseline float64
	lastAt   time.Time
	ready    bool
}

/*
NewTimeElastic returns a time-elastic baseline stage wired from config attributes on the artifact.
*/
func NewTimeElastic(artifact *datura.Artifact) *TimeElastic {
	return &TimeElastic{
		artifact: artifact,
	}
}

func (timeElastic *TimeElastic) Read(payload []byte) (int, error) {
	state := datura.Acquire("time-elastic-state", datura.APPJSON)

	if _, err := state.Unpack(timeElastic.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: state write failed",
			err,
		))
	}

	configInput := datura.Peek[string](timeElastic.artifact, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: input required",
			nil,
		))
	}

	wireRoot := datura.Peek[string](state, "root")

	if wireRoot == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: root required",
			nil,
		))
	}

	wireInputs := datura.Peek[[]string](state, "inputs")

	if len(wireInputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: inputs required",
			nil,
		))
	}

	var sample float64
	sampleFound := false

	for wireIndex, wireInput := range wireInputs {
		if wireInput != configInput {
			continue
		}

		if wireRoot == "features" {
			features := datura.Peek[[]float64](state, wireRoot)

			if wireIndex >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"time-elastic: feature index out of range",
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
			"time-elastic: input not in inputs",
			nil,
		))
	}

	halflife := time.Duration(datura.Peek[float64](timeElastic.artifact, "config", "halflife"))
	epsilon := datura.Peek[float64](timeElastic.artifact, "config", "epsilon")

	if halflife <= 0 || epsilon <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: halflife and epsilon must be positive",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: sample is non-finite",
			nil,
		))
	}

	if sample < 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: sample must be non-negative",
			nil,
		))
	}

	eventAt := time.Unix(0, int64(datura.Peek[float64](state, "at")))
	value := 1.0
	outputReady := false

	if !timeElastic.ready {
		timeElastic.baseline = sample
		timeElastic.lastAt = eventAt
		timeElastic.ready = true
	}

	if timeElastic.ready && timeElastic.lastAt != eventAt {
		if eventAt.IsZero() {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"time-elastic: event timestamp required",
				nil,
			))
		}

		delta := eventAt.Sub(timeElastic.lastAt)

		if delta < 0 {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"time-elastic: event timestamp must not regress",
				nil,
			))
		}

		timeElastic.lastAt = eventAt

		tau := float64(halflife) / math.Ln2
		alpha := 0.0

		if delta > 0 {
			alpha = 1.0 - math.Exp(-float64(delta)/tau)
		}

		value = sample / (timeElastic.baseline + epsilon)
		outputReady = sample != timeElastic.baseline
		timeElastic.baseline = (1.0-alpha)*timeElastic.baseline + alpha*sample
	}

	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: output value is non-finite",
			nil,
		))
	}

	mergeStageOutput(state, value, outputReady)

	return state.PackInto(payload)
}

func (timeElastic *TimeElastic) Write(p []byte) (int, error) {
	timeElastic.artifact.WithPlaintextPayload(p)
	return len(p), nil
}

func (timeElastic *TimeElastic) Close() error {
	return nil
}
