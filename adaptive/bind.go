package adaptive

import (
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/kernel"
)

/*
BindObserveSample returns a closure that observes raw samples through the given stages.
Returns nil when the stages are not a supported single- or two-stage fast path.
*/
func BindObserveSample(stages []core.Number) func(float64) float64 {
	switch len(stages) {
	case 1:
		return bindObserveOne(stages[0])
	case 2:
		return bindObserveTwo(stages[0], stages[1])
	default:
		return nil
	}
}

func bindObserveOne(stage core.Number) func(float64) float64 {
	switch dynamic := stage.(type) {
	case *Exponential:
		return func(raw float64) float64 {
			return kernel.ObserveEMA(&dynamic.state, raw)
		}
	case *Normalized:
		return func(raw float64) float64 {
			return kernel.ObserveDelta(&dynamic.state, raw)
		}
	case *Integrator:
		return func(raw float64) float64 {
			return kernel.ObserveAccumulator(&dynamic.state, raw)
		}
	case *Compressor:
		return func(raw float64) float64 {
			return kernel.ObserveCompression(&dynamic.state, raw)
		}
	case *Fractional:
		return func(raw float64) float64 {
			return kernel.ObserveFracDiff(&dynamic.state, raw)
		}
	case *Dispersion:
		return func(raw float64) float64 {
			return kernel.ObserveVariance(&dynamic.state, raw)
		}
	case *Surprise:
		return func(raw float64) float64 {
			return kernel.ObserveZScore(&dynamic.state, raw, 0, false)
		}
	case *Impulse:
		return func(raw float64) float64 {
			return kernel.ObserveMomentum(&dynamic.state, raw)
		}
	case *Extent:
		return func(raw float64) float64 {
			return kernel.ObserveRange(&dynamic.state, raw)
		}
	default:
		sampleDynamic, isSample := stage.(core.SampleDynamic)

		if !isSample {
			return nil
		}

		return func(raw float64) float64 {
			return sampleDynamic.ObserveSample(raw)
		}
	}
}

func bindObserveTwo(first core.Number, second core.Number) func(float64) float64 {
	switch firstDynamic := first.(type) {
	case *Exponential:
		switch secondDynamic := second.(type) {
		case *Normalized:
			return func(raw float64) float64 {
				return ObserveEMAThenDelta(raw, firstDynamic, secondDynamic)
			}
		case *Surprise:
			return func(raw float64) float64 {
				return ObserveEMAThenZScore(raw, firstDynamic, secondDynamic)
			}
		}
	case *Normalized:
		exponential, isEMA := second.(*Exponential)

		if isEMA {
			return func(raw float64) float64 {
				return ObserveEMAThenDelta(raw, exponential, firstDynamic)
			}
		}
	case *Surprise:
		exponential, isEMA := second.(*Exponential)

		if isEMA {
			return func(raw float64) float64 {
				return ObserveEMAThenZScore(raw, exponential, firstDynamic)
			}
		}
	}

	return nil
}
