package geometry

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
