package algorithm

import (
	"math"
	"time"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/correlation"
)

/*
Correlate compares synchronous Pearson and asynchronous Hayashi-Yoshida coupling
between two streams. A positive gap means async overlap exceeds aligned correlation.
*/
type Correlate struct {
	syncLeft    core.Numbers
	syncRight   core.Numbers
	asyncLeft   core.Numbers
	asyncRight  core.Numbers
	weights     core.Numbers
	maxInterval time.Duration
	pearson     *correlation.Pearson
	hayashi     *correlation.HayashiYoshida
}

/*
NewCorrelate creates a dual-correlation dynamic.
syncLeft and syncRight hold aligned scalar samples; asyncLeft and asyncRight hold
(time, value) pairs encoded as consecutive numbers per correlation.Sample.
*/
func NewCorrelate(
	syncLeft, syncRight, asyncLeft, asyncRight, weights core.Numbers,
	maxInterval time.Duration,
) *Correlate {
	return &Correlate{
		syncLeft:    syncLeft,
		syncRight:   syncRight,
		asyncLeft:   asyncLeft,
		asyncRight:  asyncRight,
		weights:     weights,
		maxInterval: maxInterval,
		pearson:     correlation.NewPearson(weights),
		hayashi:     correlation.NewHayashiYoshida(weights, maxInterval),
	}
}

/*
Observe returns the signed async-minus-sync correlation gap.
*/
func (correlate *Correlate) Observe(_ ...core.Number) core.Float64 {
	return core.Float64(correlate.Gap())
}

/*
Pearson returns the synchronous Pearson correlation.
*/
func (correlate *Correlate) Pearson() core.Float64 {
	left := nomagique.Samples(correlate.syncLeft)
	right := nomagique.Samples(correlate.syncRight)
	inputs := append(samplesToInputs(left), samplesToInputs(right)...)

	return correlate.pearson.Observe(inputs...)
}

/*
Hayashi returns the asynchronous Hayashi-Yoshida correlation.
*/
func (correlate *Correlate) Hayashi() core.Float64 {
	left := nomagique.Samples(correlate.asyncLeft)
	right := nomagique.Samples(correlate.asyncRight)
	inputs := append(samplesToInputs(left), samplesToInputs(right)...)

	return correlate.hayashi.Observe(inputs...)
}

func samplesToInputs(samples []float64) []core.Number {
	inputs := make([]core.Number, len(samples))

	for index, sample := range samples {
		inputs[index] = core.Float64(sample)
	}

	return inputs
}

/*
Gap returns Hayashi correlation minus Pearson correlation.
*/
func (correlate *Correlate) Gap() float64 {
	pearsonValue := float64(correlate.Pearson())
	hayashiValue := float64(correlate.Hayashi())

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

	if err := correlate.pearson.Reset(); err != nil {
		return err
	}

	return correlate.hayashi.Reset()
}
