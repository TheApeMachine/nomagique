package hawkes

import "math"

/*
fitFromLogParams preserves the unrestricted decoder used by optimizer tests and
the production full-model path.
*/
func fitFromLogParams(
	logParams [bivariateParamCount]float64,
	context FitContext,
) BivariateFit {
	return fitFromRestrictedLogParams(logParams, context, fitUnrestricted)
}

/*
fitFromRestrictedLogParams converts bounded log coordinates into Hawkes
parameters and applies the nested-model restriction before stability testing.
This keeps the self-only likelihood comparison on the same fitted domain as the
full model rather than zeroing coefficients after optimization.
*/
func fitFromRestrictedLogParams(
	logParams [bivariateParamCount]float64,
	context FitContext,
	restriction fitRestriction,
) BivariateFit {
	muX := math.Exp(logParams[0])
	muY := math.Exp(logParams[1])
	beta := math.Exp(logParams[2])
	branchXX := math.Exp(logParams[3])
	branchXY := math.Exp(logParams[4])
	branchYX := math.Exp(logParams[5])
	branchYY := math.Exp(logParams[6])

	if branchXX > context.BranchCeiling || branchYY > context.BranchCeiling {
		return BivariateFit{}
	}

	crossCap := context.CrossBranchCap(math.Max(branchXX, branchYY))

	if restriction == fitUnrestricted &&
		(branchXY > crossCap || branchYX > crossCap) {
		return BivariateFit{}
	}

	fit := BivariateFit{
		MuX:     muX,
		MuY:     muY,
		AlphaXX: branchXX * beta,
		AlphaXY: branchXY * beta,
		AlphaYX: branchYX * beta,
		AlphaYY: branchYY * beta,
		Beta:    beta,
	}

	if restriction == fitSelfOnly {
		fit.AlphaXY = 0
		fit.AlphaYX = 0
	}

	fit.SpectralRadius = fit.computeSpectralRadius()

	if fit.SpectralRadius < 0 || fit.SpectralRadius >= criticalBranch {
		return BivariateFit{}
	}

	return fit
}
