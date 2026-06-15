package logic

import (
	"github.com/theapemachine/nomagique/core"
)

func collectScalars[T ~float64](inputs ...core.Number[T]) ([]float64, bool) {
	scalars := make([]float64, 0, len(inputs))

	for _, input := range inputs {
		scalar, ok := input.(core.Scalar[T])

		if !ok {
			return nil, false
		}

		scalars = append(scalars, float64(scalar))
	}

	return scalars, true
}

func truthy(sample float64) bool {
	return sample > 0
}

func scalarInputs[T ~float64](values ...float64) []core.Number[T] {
	inputs := make([]core.Number[T], len(values))

	for index, value := range values {
		inputs[index] = core.Scalar[T](T(value))
	}

	return inputs
}
