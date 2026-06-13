package adaptive

import (
	"github.com/theapemachine/nomagique/core"
)

var (
	_ core.SampleDynamic = (*Exponential)(nil)
	_ core.SampleDynamic = (*Normalized)(nil)
	_ core.SampleDynamic = (*Integrator)(nil)
	_ core.SampleDynamic = (*Compressor)(nil)
	_ core.SampleDynamic = (*Fractional)(nil)
	_ core.SampleDynamic = (*Dispersion)(nil)
	_ core.SampleDynamic = (*Surprise)(nil)
	_ core.SampleDynamic = (*Impulse)(nil)
	_ core.SampleDynamic = (*Extent)(nil)
)

/*
ObserveSample ingests a raw float64 sample into the EMA state.
*/
func (exponential *Exponential) ObserveSample(sample float64) float64 {
	return ObserveEMA(&exponential.state, sample)
}

/*
ObserveSample ingests a raw float64 sample into the delta state.
*/
func (delta *Normalized) ObserveSample(sample float64) float64 {
	return ObserveDelta(&delta.state, sample)
}

/*
ObserveSample ingests a raw float64 sample into the accumulator level.
*/
func (integrator *Integrator) ObserveSample(sample float64) float64 {
	return ObserveAccumulator(&integrator.state, sample)
}

/*
ObserveSample ingests a raw float64 sample into the compression state.
*/
func (compressor *Compressor) ObserveSample(sample float64) float64 {
	return ObserveCompression(&compressor.state, sample)
}

/*
ObserveSample ingests a raw float64 sample into the fractional differencing state.
*/
func (fractional *Fractional) ObserveSample(sample float64) float64 {
	return ObserveFracDiff(&fractional.state, sample)
}

/*
ObserveSample ingests a raw float64 sample into the variance state.
*/
func (dispersion *Dispersion) ObserveSample(sample float64) float64 {
	return ObserveVariance(&dispersion.state, sample)
}

/*
ObserveSample ingests a raw float64 sample into the z-score state.
*/
func (surprise *Surprise) ObserveSample(sample float64) float64 {
	return ObserveZScore(&surprise.state, sample, 0, false)
}

/*
ObserveSample ingests a raw float64 sample into the momentum state.
*/
func (impulse *Impulse) ObserveSample(sample float64) float64 {
	return ObserveMomentum(&impulse.state, sample)
}

/*
ObserveSample ingests a raw float64 sample into the range state.
*/
func (extent *Extent) ObserveSample(sample float64) float64 {
	return ObserveRange(&extent.state, sample)
}

/*
ObserveEMAThenZScore runs EMA then z-score with the EMA level as anchor.
Matches pipeline ordering when work carries the smoothed level.
*/
func ObserveEMAThenZScore(
	raw float64, exponential *Exponential, surprise *Surprise,
) float64 {
	anchor := float64(exponential.state.Value)

	if !exponential.state.Ready {
		anchor = raw
	}

	ObserveEMA(&exponential.state, raw)

	return ObserveZScore(&surprise.state, raw, anchor, true)
}

/*
ObserveEMAThenDelta runs EMA then delta on the same raw sample.
Matches core.Pipeline ordering for those two stages.
*/
func ObserveEMAThenDelta(
	raw float64, exponential *Exponential, delta *Normalized,
) float64 {
	ObserveEMA(&exponential.state, raw)

	return ObserveDelta(&delta.state, raw)
}
