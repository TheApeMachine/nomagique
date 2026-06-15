package tests

import (
	"math"

	"github.com/theapemachine/nomagique/core"
)

/*
PipelineStage is a non-scalar Number input for adversarial Observe tests.
*/
type PipelineStage[T ~float64] struct {
	Result core.Scalar[T]
}

func (stage *PipelineStage[T]) Observe(...core.Number[T]) core.Scalar[T] {
	return stage.Result
}

func (stage *PipelineStage[T]) Reset() error {
	return nil
}

/*
RunObserveSampleSequence feeds samples through ObserveSample and returns the last value.
*/
func RunObserveSampleSequence(run func(sample float64) float64, samples []float64) float64 {
	var last float64

	for _, sample := range samples {
		last = run(sample)
	}

	return last
}

/*
RunObserveScalarSequence feeds samples through Observe scalars and returns the last value.
*/
func RunObserveScalarSequence(
	observe func(inputs ...core.Number[float64]) core.Scalar[float64],
	samples []float64,
) float64 {
	var last float64

	for _, sample := range samples {
		last = float64(observe(core.Scalar[float64](sample)))
	}

	return last
}

/*
AlmostEqual compares floats with relative tolerance, treating NaN pairs as equal.
*/
func AlmostEqual(left float64, right float64) bool {
	if math.IsNaN(left) && math.IsNaN(right) {
		return true
	}

	return math.Abs(left-right) <= 1e-12*math.Max(1, math.Max(math.Abs(left), math.Abs(right)))
}
