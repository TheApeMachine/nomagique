package adaptive

import (
	"math"
	"time"

	"github.com/theapemachine/datura"
)

/*
TimeElastic tracks a time-decayed baseline and returns sample/baseline ratios.
*/
type TimeElastic struct {
	artifact *datura.Artifact
}

/*
NewTimeElastic returns a time-elastic baseline stage.
*/
func NewTimeElastic(halflife time.Duration, epsilon float64) *TimeElastic {
	if epsilon <= 0 {
		epsilon = math.Sqrt(math.Nextafter(1, 2) - 1)
	}

	stage := &TimeElastic{
		artifact: datura.Acquire("time_elastic", datura.APPJSON).RetainStageAttributes(),
	}

	stage.artifact.Poke(float64(halflife), "halflife")
	stage.artifact.Poke(epsilon, "epsilon")

	return stage
}

func (timeElastic *TimeElastic) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](timeElastic.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return timeElastic.artifact.Read(p)
	}

	if sample < 0 {
		return timeElastic.artifact.Read(p)
	}

	eventAt := time.Unix(0, int64(datura.Peek[float64](timeElastic.artifact, "at")))
	halflife := time.Duration(datura.Peek[float64](timeElastic.artifact, "halflife"))
	epsilon := datura.Peek[float64](timeElastic.artifact, "epsilon")

	output := datura.Peek[datura.Map[float64]](timeElastic.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"baseline": sample,
			"lastAt":   float64(eventAt.UnixNano()),
			"value":    1,
		}

		timeElastic.artifact.Poke(output, "output")

		return timeElastic.artifact.Read(p)
	}

	if halflife <= 0 || eventAt.IsZero() {
		return timeElastic.artifact.Read(p)
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

	timeElastic.artifact.Poke(output, "output")

	return timeElastic.artifact.Read(p)
}

func (timeElastic *TimeElastic) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](timeElastic.artifact, "output") == nil

	timeElastic.artifact.Clear("sample")
	timeElastic.artifact.Clear("at")

	n, err := timeElastic.artifact.Write(p)

	if bootstrap {
		timeElastic.artifact.Clear("output")
	}

	return n, err
}

func (timeElastic *TimeElastic) Close() error {
	return nil
}
