/*
Package nomagique implements a derived-signal runtime: numbers emerge from
observations and composable dynamics, not from fixed coefficients or magic constants.
*/
package nomagique

import (
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/core"
)

/*
Scalar is a boundary float. Use += for a raw sample, then Observe with dynamics.
*/
type Scalar float64

/*
Observe applies the given dynamics to the receiver as raw input.
*/
func (scalar Scalar) Observe(stages ...core.Number) core.Float64 {
	raw := float64(scalar)

	if apply, ok := stageObserveApply(stages); ok {
		return core.Float64(apply(raw))
	}

	if apply, ok := boundObserveApply(core.Float64(scalar), stages); ok {
		return core.Float64(apply(raw))
	}

	if result, ok := scalar.observeFast(raw, stages); ok {
		return result
	}

	pipeline := core.AcquirePipeline(scalar.resolveStages(stages))

	result, err := pipeline.Observe(core.Float64(raw))

	core.ReleasePipeline(pipeline)

	if err != nil {
		return 0
	}

	core.DefaultRegistry.Register(result, pipeline)

	return result
}

/*
Reset is a no-op at the boundary scalar.
*/
func (scalar Scalar) Reset() error {
	return nil
}

/*
Number returns a boundary scalar wired to the given dynamics.
*/
func Number(stages ...core.Number) (Scalar, error) {
	resolved := resolveStages(stages)

	result, err := observeResolved(0, resolved)

	if err != nil {
		return 0, err
	}

	core.DefaultRegistry.RegisterStages(result, resolved)
	registerObserveBinder(result, resolved)

	return Scalar(result), nil
}

func Numbers(series ...float64) core.Numbers {
	numbers := make([]core.Number, len(series))

	for index, sample := range series {
		numbers[index] = Scalar(sample)
	}

	return numbers
}

/*
Samples reads raw float64 observations from boundary numbers without applying dynamics.
*/
func Samples(numbers core.Numbers) []float64 {
	samples := make([]float64, len(numbers))

	for index, number := range numbers {
		switch value := number.(type) {
		case Scalar:
			samples[index] = float64(value)
		case core.Float64:
			samples[index] = float64(value)
		default:
			samples[index] = float64(number.Observe())
		}
	}

	return samples
}

func observeResolved(
	raw float64, stages []core.Number,
) (core.Float64, error) {
	scalar := Scalar(raw)

	if result, ok := scalar.observeFast(raw, stages); ok {
		return result, nil
	}

	pipeline := core.AcquirePipeline(stages)

	result, err := pipeline.Observe(core.Float64(raw))

	core.ReleasePipeline(pipeline)

	if err != nil {
		return 0, err
	}

	core.DefaultRegistry.Register(result, pipeline)

	return result, nil
}

func (scalar Scalar) observeFast(
	raw float64, stages []core.Number,
) (core.Float64, bool) {
	switch len(stages) {
	case 1:
		return scalar.observeOne(raw, stages[0])
	case 2:
		return scalar.observeTwo(raw, stages[0], stages[1])
	default:
		return 0, false
	}
}

func (scalar Scalar) observeOne(
	raw float64, stage core.Number,
) (core.Float64, bool) {
	sampleDynamic, isSample := stage.(core.SampleDynamic)

	if isSample {
		return core.Float64(sampleDynamic.ObserveSample(raw)), true
	}

	nestedScalar, isScalar := stage.(Scalar)

	if !isScalar {
		return 0, false
	}

	nested, registered := core.DefaultRegistry.StagesFor(core.Float64(nestedScalar))

	if !registered {
		return 0, false
	}

	return scalar.observeFast(raw, nested)
}

func (scalar Scalar) observeTwo(
	raw float64, first core.Number, second core.Number,
) (core.Float64, bool) {
	switch firstDynamic := first.(type) {
	case *adaptive.Exponential:
		switch secondDynamic := second.(type) {
		case *adaptive.Normalized:
			return core.Float64(
				adaptive.ObserveEMAThenDelta(raw, firstDynamic, secondDynamic),
			), true
		case *adaptive.Surprise:
			return core.Float64(
				adaptive.ObserveEMAThenZScore(raw, firstDynamic, secondDynamic),
			), true
		}
	case *adaptive.Normalized:
		exponential, isEMA := second.(*adaptive.Exponential)

		if isEMA {
			return core.Float64(
				adaptive.ObserveEMAThenDelta(raw, exponential, firstDynamic),
			), true
		}
	case *adaptive.Surprise:
		exponential, isEMA := second.(*adaptive.Exponential)

		if isEMA {
			return core.Float64(
				adaptive.ObserveEMAThenZScore(raw, exponential, firstDynamic),
			), true
		}
	}

	return 0, false
}

func resolveStages(stages []core.Number) []core.Number {
	flattened := stageSlicePool.Get().([]core.Number)
	flattened = flattened[:0]

	for _, stage := range stages {
		nestedScalar, isScalar := stage.(Scalar)

		if isScalar {
			token := core.Float64(nestedScalar)
			nested, registered := core.DefaultRegistry.StagesFor(token)

			if registered {
				flattened = append(flattened, nested...)
				continue
			}
		}

		flattened = append(flattened, stage)
	}

	expanded := core.DefaultRegistry.ExpandNumbers(flattened)
	stageSlicePool.Put(flattened)

	return expanded
}

func (scalar Scalar) resolveStages(stages []core.Number) []core.Number {
	return resolveStages(stages)
}
