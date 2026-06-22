package adaptive

import (
	"math"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

/*
TimeElastic tracks a time-decayed baseline and returns sample/baseline ratios.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type TimeElastic struct {
	artifact *datura.Artifact
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

func (timeElastic *TimeElastic) Write(payload []byte) (int, error) {
	timeElastic.artifact.WithPayload(payload)
	return len(payload), nil
}

func (timeElastic *TimeElastic) Read(payload []byte) (int, error) {
	state := datura.Acquire("time-elastic-state", datura.APPJSON)
	state.Inspect("adaptive", "time-elastic", "Read()", "p")

	if _, err := state.Write(timeElastic.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	features := statistic.SnapshotFeatures(state)
	sampleKey := statistic.WireInputKey(timeElastic.artifact, state, "sample")
	sample, err := statistic.WireScalar(timeElastic.artifact, state, sampleKey)

	if err != nil {
		return 0, err
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
	output := datura.Peek[datura.Map[float64]](timeElastic.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"baseline": sample,
			"lastAt":   float64(eventAt.UnixNano()),
			"value":    1,
		}

		timeElastic.artifact.Poke(output, "output")
		state.MergeOutput("value", output["value"])
		features.Restore(state)
		state.Merge("root", "output")

		if len(datura.Peek[[]string](state, "inputs")) == 0 {
			state.Merge("inputs", []string{"value"})
		}

		return state.Read(payload)
	}

	if eventAt.IsZero() {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: event timestamp required",
			nil,
		))
	}

	lastAt := time.Unix(0, int64(output["lastAt"]))
	delta := eventAt.Sub(lastAt)

	if delta < 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: event timestamp must not regress",
			nil,
		))
	}

	output["lastAt"] = float64(eventAt.UnixNano())

	tau := float64(halflife) / math.Ln2

	var alpha float64

	if delta > 0 {
		alpha = 1.0 - math.Exp(-float64(delta)/tau)
	}

	output["value"] = sample / (output["baseline"] + epsilon)
	output["baseline"] = (1.0-alpha)*output["baseline"] + alpha*sample

	if math.IsNaN(output["value"]) || math.IsInf(output["value"], 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"time-elastic: output value is non-finite",
			nil,
		))
	}

	timeElastic.artifact.Poke(output, "output")
	state.MergeOutput("value", output["value"])
	features.Restore(state)
	state.Merge("root", "output")

	if len(datura.Peek[[]string](state, "inputs")) == 0 {
		state.Merge("inputs", []string{"value"})
	}

	return state.Read(payload)
}

func (timeElastic *TimeElastic) Close() error {
	return nil
}
