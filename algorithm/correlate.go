package algorithm

import (
	"math"
	"time"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/stat"
)

/*
Correlate compares synchronous Pearson and asynchronous Hayashi-Yoshida coupling
between two streams. A positive gap means async overlap exceeds aligned correlation.
*/
type Correlate struct {
	artifact    *datura.Artifact
	syncLeft    []float64
	syncRight   []float64
	asyncLeft   []float64
	asyncRight  []float64
	weights     []float64
	maxInterval time.Duration
}

/*
NewCorrelate creates a dual-correlation stage.
syncLeft and syncRight hold aligned scalar samples; asyncLeft and asyncRight hold
(time, value) pairs encoded as consecutive numbers per correlation.Sample.
*/
func NewCorrelate(
	syncLeft, syncRight, asyncLeft, asyncRight, weights []float64,
	maxInterval time.Duration,
) *Correlate {
	return &Correlate{
		artifact:    datura.Acquire("correlate", datura.Artifact_Type_json),
		syncLeft:    syncLeft,
		syncRight:   syncRight,
		asyncLeft:   asyncLeft,
		asyncRight:  asyncRight,
		weights:     weights,
		maxInterval: maxInterval,
	}
}

func (correlate *Correlate) Write(p []byte) (int, error) {
	return correlate.artifact.Write(p)
}

func (correlate *Correlate) Read(p []byte) (int, error) {
	rehydrateArtifact(&correlate.artifact, "correlate", datura.Artifact_Type_json)

	gap := correlate.Gap()
	out := encodePayload(gap)
	_ = correlate.artifact.SetPayload(out)

	return correlate.artifact.Read(p)
}

func (correlate *Correlate) Close() error {
	return nil
}

/*
Pearson returns the synchronous Pearson correlation.
*/
func (correlate *Correlate) Pearson() float64 {
	if len(correlate.syncLeft) < 2 || len(correlate.syncRight) < 2 {
		return 0
	}

	if len(correlate.syncLeft) != len(correlate.syncRight) {
		return 0
	}

	weights, weightsOK := weightSamplesFor(correlate.weights, len(correlate.syncLeft))

	if !weightsOK {
		return 0
	}

	correlationValue := stat.Correlation(
		correlate.syncLeft,
		correlate.syncRight,
		weights,
	)

	if math.IsNaN(correlationValue) || math.IsInf(correlationValue, 0) {
		return 0
	}

	return correlationValue
}

/*
Hayashi returns the asynchronous Hayashi-Yoshida correlation.
*/
func (correlate *Correlate) Hayashi() float64 {
	left, leftOK := samplesFromTimeValues(correlate.asyncLeft)
	right, rightOK := samplesFromTimeValues(correlate.asyncRight)

	if !leftOK || !rightOK {
		return 0
	}

	correlationValue, ok := hayashiCorrelation(left, right, correlate.maxInterval)

	if !ok {
		return 0
	}

	return correlationValue
}

/*
Gap returns Hayashi correlation minus Pearson correlation.
*/
func (correlate *Correlate) Gap() float64 {
	pearsonValue := correlate.Pearson()
	hayashiValue := correlate.Hayashi()

	if math.IsNaN(pearsonValue) || math.IsNaN(hayashiValue) {
		return 0
	}

	return hayashiValue - pearsonValue
}

/*
Reset clears derived state.
*/
func (correlate *Correlate) Reset() error {
	correlate.weights = nil

	return nil
}
