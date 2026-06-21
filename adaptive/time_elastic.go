package adaptive

import (
	"math"
	"time"

	"github.com/theapemachine/datura"
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
	output := datura.Peek[datura.Map[float64]](timeElastic.artifact, "output")

	timeElastic.artifact.WithPayload(payload)

	if output != nil {
		timeElastic.artifact.Merge("output", output)
	}

	return len(payload), nil
}

func (timeElastic *TimeElastic) Read(payload []byte) (int, error) {
	state := datura.Acquire("time-elastic-state", datura.APPJSON)
	state.Inspect("adaptive", "time-elastic", "Read()", "p")

	if _, err := state.Write(timeElastic.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return state.Read(payload)
	}

	if sample < 0 {
		return state.Read(payload)
	}

	eventAt := time.Unix(0, int64(datura.Peek[float64](state, "at")))
	halflife := time.Duration(datura.Peek[float64](timeElastic.artifact, "config", "halflife"))
	epsilon := datura.Peek[float64](timeElastic.artifact, "config", "epsilon")

	if epsilon <= 0 {
		epsilon = math.Sqrt(math.Nextafter(1, 2) - 1)
	}

	output := datura.Peek[datura.Map[float64]](timeElastic.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"baseline": sample,
			"lastAt":   float64(eventAt.UnixNano()),
			"value":    1,
		}

		timeElastic.artifact.Merge("output", output)
		state.MergeOutput("value", output["value"])
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(payload)
	}

	if halflife <= 0 || eventAt.IsZero() {
		return state.Read(payload)
	}

	lastAt := time.Unix(0, int64(output["lastAt"]))
	delta := eventAt.Sub(lastAt)

	if delta < 0 {
		delta = 0
	}

	output["lastAt"] = float64(eventAt.UnixNano())

	tau := float64(halflife) / math.Ln2

	var alpha float64

	if tau > 0 && delta > 0 {
		alpha = 1.0 - math.Exp(-float64(delta)/tau)
	}

	if delta > 0 && tau <= 0 {
		alpha = 1.0
	}

	output["value"] = sample / (output["baseline"] + epsilon)
	output["baseline"] = (1.0-alpha)*output["baseline"] + alpha*sample

	timeElastic.artifact.Merge("output", output)
	state.MergeOutput("value", output["value"])
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (timeElastic *TimeElastic) Close() error {
	return nil
}
