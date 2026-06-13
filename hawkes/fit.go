package hawkes

import (
	"math"
	"time"
)

const criticalBranch = 1.0

/*
BivariateFit holds joint Hawkes MLE parameters and horizon intensities.
*/
type BivariateFit struct {
	MuX            float64
	MuY            float64
	AlphaXX        float64
	AlphaXY        float64
	AlphaYX        float64
	AlphaYY        float64
	Beta           float64
	IntensityX     float64
	IntensityY     float64
	SpectralRadius float64
}

/*
Valid reports whether fit parameters are positive and subcritical.
*/
func (fit BivariateFit) Valid() bool {
	return fit.MuX > 0 &&
		fit.MuY > 0 &&
		fit.Beta > 0 &&
		fit.AlphaXX >= 0 &&
		fit.AlphaXY >= 0 &&
		fit.AlphaYX >= 0 &&
		fit.AlphaYY >= 0 &&
		fit.SpectralRadius > 0 &&
		fit.SpectralRadius < criticalBranch
}

/*
Params converts the fit to count-stream BivariateParams.
*/
func (fit BivariateFit) Params() BivariateParams {
	return BivariateParams{
		MuX:     fit.MuX,
		MuY:     fit.MuY,
		AlphaXX: fit.AlphaXX,
		AlphaXY: fit.AlphaXY,
		AlphaYX: fit.AlphaYX,
		AlphaYY: fit.AlphaYY,
		Beta:    fit.Beta,
	}
}

func (fit BivariateFit) computeSpectralRadius() float64 {
	if fit.Beta <= 0 {
		return math.Inf(1)
	}

	branching := fit.Params().branchingMatrix()

	return SpectralRadius(branching)
}

/*
LogLikelihood returns the exact log-likelihood at horizon.
*/
func (fit BivariateFit) LogLikelihood(stream ArrivalStream, horizon time.Time) float64 {
	if fit.MuX <= 0 || fit.MuY <= 0 || fit.Beta <= 0 {
		return math.Inf(-1)
	}

	if fit.AlphaXX < 0 || fit.AlphaXY < 0 || fit.AlphaYX < 0 || fit.AlphaYY < 0 {
		return math.Inf(-1)
	}

	marked := stream.Marked()

	if len(marked) == 0 {
		return math.Inf(-1)
	}

	span := stream.Span(horizon)

	if span <= 0 {
		return math.Inf(-1)
	}

	state := ExcitationState{}
	logSum, ok := state.LogLikelihoodSum(
		marked,
		fit.MuX, fit.MuY,
		fit.AlphaXX, fit.AlphaXY, fit.AlphaYX, fit.AlphaYY,
		fit.Beta,
	)

	if !ok {
		return math.Inf(-1)
	}

	compensator := fit.compensator(stream, horizon, span)

	return logSum - compensator
}

/*
WithIntensitiesAt attaches horizon intensities to the fit.
*/
func (fit BivariateFit) WithIntensitiesAt(stream ArrivalStream, horizon time.Time) BivariateFit {
	result := fit
	result.IntensityX = stream.buyIntensityAt(
		horizon, fit.MuX, fit.AlphaXX, fit.AlphaXY, fit.Beta,
	)
	result.IntensityY = stream.sellIntensityAt(
		horizon, fit.MuY, fit.AlphaYX, fit.AlphaYY, fit.Beta,
	)

	return result
}

/*
Asymmetry returns normalized intensity excess on the requested side.
*/
func (fit BivariateFit) Asymmetry(preferY bool) float64 {
	total := fit.IntensityX + fit.IntensityY

	if total <= 0 {
		return 0
	}

	if preferY {
		if fit.IntensityY <= fit.IntensityX {
			return 0
		}

		return (fit.IntensityY - fit.IntensityX) / total
	}

	if fit.IntensityX <= fit.IntensityY {
		return 0
	}

	return (fit.IntensityX - fit.IntensityY) / total
}

/*
Runway is the fitted kernel e-folding time 1/beta.
*/
func (fit BivariateFit) Runway() time.Duration {
	if fit.Beta <= 0 {
		return 0
	}

	return time.Duration((1 / fit.Beta) * float64(time.Second))
}

/*
ExcitationConfidence scores excitation ratio weighted by asymmetry.
*/
func (fit BivariateFit) ExcitationConfidence(
	asymmetry float64,
	baselineFence float64,
	preferY bool,
) float64 {
	if asymmetry <= 0 || fit.SpectralRadius >= criticalBranch {
		return 0
	}

	if preferY {
		if fit.MuY <= 0 || fit.IntensityY <= 0 {
			return 0
		}

		ratio := fit.IntensityY / fit.MuY

		if ratio <= baselineFence {
			return 0
		}

		return asymmetry * ratio
	}

	if fit.MuX <= 0 || fit.IntensityX <= 0 {
		return 0
	}

	ratio := fit.IntensityX / fit.MuX

	if ratio <= baselineFence {
		return 0
	}

	return asymmetry * ratio
}

/*
ClampSubcritical scales excitation parameters to stay below criticalBranch.
*/
func (fit BivariateFit) ClampSubcritical() BivariateFit {
	if fit.SpectralRadius <= 0 || fit.SpectralRadius >= criticalBranch {
		return fit
	}

	factor := criticalBranch / fit.SpectralRadius

	if factor >= 1 {
		return fit
	}

	clamped := fit
	clamped.AlphaXX *= factor
	clamped.AlphaXY *= factor
	clamped.AlphaYX *= factor
	clamped.AlphaYY *= factor
	clamped.SpectralRadius = clamped.computeSpectralRadius()

	return clamped
}

func (fit BivariateFit) withCrossZeroed() BivariateFit {
	if fit.AlphaXY <= 0 && fit.AlphaYX <= 0 {
		return fit
	}

	restricted := fit
	restricted.AlphaXY = 0
	restricted.AlphaYX = 0
	restricted.SpectralRadius = restricted.computeSpectralRadius()

	return restricted
}

func (fit BivariateFit) compensator(
	stream ArrivalStream,
	horizon time.Time,
	span float64,
) float64 {
	beta := fit.Beta
	buySupport, sellSupport := stream.kernelSupport(horizon, beta)

	buyIntegral := fit.MuX*span +
		(fit.AlphaXX/beta)*buySupport +
		(fit.AlphaXY/beta)*sellSupport
	sellIntegral := fit.MuY*span +
		(fit.AlphaYX/beta)*buySupport +
		(fit.AlphaYY/beta)*sellSupport

	return buyIntegral + sellIntegral
}
