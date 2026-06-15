package statistic

import (
	"github.com/theapemachine/nomagique/core"
)

func sampleBatch[T ~float64](inputs ...core.Number[T]) []float64 {
	values := make([]float64, 0, len(inputs))

	for _, input := range inputs {
		scalar, ok := input.(core.Scalar[T])

		if !ok {
			continue
		}

		values = append(values, float64(scalar))
	}

	return values
}

func pairScalars[T ~float64](inputs ...core.Number[T]) (float64, float64, bool) {
	values := sampleBatch[T](inputs...)

	if len(values) < 2 {
		return 0, 0, false
	}

	return values[0], values[1], true
}

func boundarySample[T ~float64](inputs ...core.Number[T]) (float64, bool) {
	if len(inputs) == 0 {
		return 0, false
	}

	sample, ok := inputs[0].(core.Scalar[T])

	if !ok {
		return 0, false
	}

	if len(inputs) > 1 {
		if work, workOK := inputs[1].(core.Scalar[T]); workOK {
			sample = core.Scalar[T](T(sample) + T(work))
		}
	}

	return float64(sample), true
}
