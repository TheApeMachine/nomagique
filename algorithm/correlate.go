package algorithm

import (
	"math"
	"time"

	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/correlation"
)

/*
Correlate compares synchronous Pearson and asynchronous Hayashi-Yoshida coupling
between two streams. A positive gap means async overlap exceeds aligned correlation.
*/
type Correlate[T ~float64] struct {
	syncLeft    []float64
	syncRight   []float64
	asyncLeft   []float64
	asyncRight  []float64
	weights     []float64
	maxInterval time.Duration
	pearson     *correlation.Pearson[T]
	hayashi     *correlation.HayashiYoshida[T]
	output      core.Scalar[T]
}

/*
NewCorrelate creates a dual-correlation dynamic.
syncLeft and syncRight hold aligned scalar samples; asyncLeft and asyncRight hold
(time, value) pairs encoded as consecutive numbers per correlation.Sample.
*/
func NewCorrelate[T ~float64](
	syncLeft, syncRight, asyncLeft, asyncRight, weights []float64,
	maxInterval time.Duration,
) *Correlate[T] {
	return &Correlate[T]{
		syncLeft:    syncLeft,
		syncRight:   syncRight,
		asyncLeft:   asyncLeft,
		asyncRight:  asyncRight,
		weights:     weights,
		maxInterval: maxInterval,
		pearson:     correlation.NewPearson[T](weights),
		hayashi:     correlation.NewHayashiYoshida[T](weights, maxInterval),
	}
}

/*
Observe returns the signed async-minus-sync correlation gap.
*/
func (correlate *Correlate[T]) Observe(_ ...core.Number[T]) core.Scalar[T] {
	correlate.output = core.Scalar[T](T(correlate.Gap()))

	return correlate.output
}

/*
Pearson returns the synchronous Pearson correlation.
*/
func (correlate *Correlate[T]) Pearson() core.Scalar[T] {
	inputs := append(
		samplesToInputs[T](correlate.syncLeft),
		samplesToInputs[T](correlate.syncRight)...,
	)

	return correlate.pearson.Observe(inputs...)
}

/*
Hayashi returns the asynchronous Hayashi-Yoshida correlation.
*/
func (correlate *Correlate[T]) Hayashi() core.Scalar[T] {
	inputs := append(
		samplesToInputs[T](correlate.asyncLeft),
		samplesToInputs[T](correlate.asyncRight)...,
	)

	return correlate.hayashi.Observe(inputs...)
}

/*
Gap returns Hayashi correlation minus Pearson correlation.
*/
func (correlate *Correlate[T]) Gap() float64 {
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
func (correlate *Correlate[T]) Reset() error {
	correlate.weights = nil
	correlate.output = core.Scalar[T](0)

	if err := correlate.pearson.Reset(); err != nil {
		return err
	}

	return correlate.hayashi.Reset()
}
