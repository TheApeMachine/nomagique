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
	artifact.Inspect("adaptive", "time-elastic", "NewTimeElastic()")

	return &TimeElastic{
		artifact: artifact,
	}
}

func (timeElastic *TimeElastic) Write(p []byte) (int, error) {
	timeElastic.artifact.WithPayload(p)
	return len(p), nil
}

func (timeElastic *TimeElastic) Read(payload []byte) (int, error) {
	state := datura.Acquire("time-elastic-state", datura.APPJSON)
	state.Inspect("adaptive", "time-elastic", "Read()", "p")

	if _, err := state.Write(timeElastic.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: state write failed",
			err,
		))
	}

	root := datura.Peek[string](state, "root")
	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		if root == "output" {
			inputs = []string{"value"}
		} else {
			inputs = []string{"sample"}
		}
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

	eventAt := time.Unix(0, int64(datura.Peek[float64](state, "at")))
	value := 1.0

	for _, input := range inputs {
		sample := datura.Peek[float64](state, root, input)

		if root == "" {
			sample = datura.Peek[float64](state, input)
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

		if !timeElastic.ready {
			timeElastic.baseline = sample
			timeElastic.lastAt = eventAt
			timeElastic.ready = true
			value = 1

			continue
		}

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
		timeElastic.baseline = (1.0-alpha)*timeElastic.baseline + alpha*sample
	}

	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: output value is non-finite",
			nil,
		))
	}

	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")
	state.MergeOutput("value", value)

	return state.Read(payload)
}

func (timeElastic *TimeElastic) Close() error {
	return nil
}
