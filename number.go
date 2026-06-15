package nomagique

import (
	"github.com/theapemachine/nomagique/core"
)

/*
Number composes adaptive pipeline stages into a single derived sample. It is
the intended entry point for highly dynamic environments where fixed thresholds,
tuned constants, and other static parameters fail: nomagique is a library of
composable primitives meant to be wired together into larger algorithmic structures,
each stage deriving its behavior from the observations it receives.

Compose at the source through Number and the stage types directly. Wrapping it
in configuration layers, indirection, or external control planes tends to
reintroduce the same magic numbers and static assumptions this package avoids.
*/
func Number[T ~float64](stages ...core.Number[T]) float64 {
	return float64(core.Scalar[T](0).Observe(stages...))
}

/*
Numbers wraps raw samples as pipeline inputs. Pass the result into stages that
expect multiple observations—for example, correlation or cross-section
dynamics—without introducing intermediate configuration or static shaping of
the series.
*/
func Numbers[T ~float64](series ...T) []core.Number[T] {
	numbers := make([]core.Number[T], len(series))

	for index, sample := range series {
		numbers[index] = core.Scalar[T](sample)
	}

	return numbers
}
