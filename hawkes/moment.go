package hawkes

import (
	"math"

	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
Moment validates bivariate exponential-kernel parameters through empirical moments.
*/
type Moment struct {
	config MomentConfig
}

/*
MomentConfig selects the mixed moment used for fit diagnostics.
*/
type MomentConfig struct {
	Params  BivariateParams
	MomentR float64
	MomentS float64
}

/*
MomentInput carries aligned count streams.
*/
type MomentInput struct {
	X       []float64
	Y       []float64
	Weights []float64
}

/*
MomentOutput carries empirical and estimated moment evidence.
*/
type MomentOutput struct {
	Value      float64
	Empirical  float64
	Estimate   float64
	Confidence float64
}

/*
NewMoment creates a typed Hawkes moment diagnostic.
*/
func NewMoment(config MomentConfig) (*Moment, error) {
	if config.MomentR < 0 || config.MomentS < 0 || config.MomentR+config.MomentS == 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes-moment: config momentR and momentS must define a nonzero moment",
			nil,
		))
	}

	return &Moment{config: config}, nil
}

/*
Measure evaluates empirical moments against configured Hawkes parameters.
*/
func (moment *Moment) Measure(input MomentInput) (MomentOutput, error) {
	if len(input.X) != len(input.Y) || len(input.X) < 2 {
		return MomentOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes-moment: require aligned sample streams of at least two observations",
			nil,
		))
	}

	if len(input.Weights) != 0 && len(input.Weights) != len(input.X) {
		return MomentOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes-moment: weights must align with sample streams",
			nil,
		))
	}

	empirical := stat.BivariateMoment(
		moment.config.MomentR,
		moment.config.MomentS,
		input.X,
		input.Y,
		input.Weights,
	)
	estimate, estimateOK := BranchingMomentEstimate(
		moment.config.Params,
		moment.config.MomentR,
		moment.config.MomentS,
	)

	if !estimateOK {
		return MomentOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes-moment: branching moment estimate unavailable for parameters",
			nil,
		))
	}

	confidence, confidenceOK := MomentConfidence(empirical, estimate)

	if !confidenceOK {
		return MomentOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes-moment: confidence could not be derived",
			nil,
		))
	}

	return MomentOutput{
		Value:      confidence,
		Empirical:  empirical,
		Estimate:   estimate,
		Confidence: confidence,
	}, nil
}

/*
MethodOfMoments derives a stable seed from empirical x and y count streams.
*/
func MethodOfMoments(
	x, y, weights []float64, beta float64,
) (BivariateParams, bool) {
	if beta <= 0 || len(x) != len(y) || len(x) < 2 {
		return BivariateParams{}, false
	}

	meanX := stat.Mean(x, weights)
	meanY := stat.Mean(y, weights)

	if meanX <= 0 || meanY <= 0 {
		return BivariateParams{}, false
	}

	secondMomentX := stat.BivariateMoment(2, 0, x, y, weights)
	secondMomentY := stat.BivariateMoment(0, 2, x, y, weights)
	centralVarianceX := secondMomentX - meanX*meanX
	centralVarianceY := secondMomentY - meanY*meanY
	covariance := stat.BivariateMoment(1, 1, x, y, weights)

	params := BivariateParams{Beta: beta}

	if centralVarianceX > meanX {
		params.AlphaXX = (centralVarianceX - meanX) * beta / (2 * meanX)
	}

	if centralVarianceY > meanY {
		params.AlphaYY = (centralVarianceY - meanY) * beta / (2 * meanY)
	}

	if covariance > 0 {
		params.AlphaXY = covariance * beta / (2 * meanY)
		params.AlphaYX = covariance * beta / (2 * meanX)
	}

	branchXX := params.AlphaXX / beta
	branchXY := params.AlphaXY / beta
	branchYX := params.AlphaYX / beta
	branchYY := params.AlphaYY / beta

	params.MuX = meanX - branchXX*meanX - branchXY*meanY
	params.MuY = meanY - branchYX*meanX - branchYY*meanY

	if params.MuX <= 0 || params.MuY <= 0 || !params.Stable() {
		return BivariateParams{}, false
	}

	return params, true
}

/*
BranchingMomentEstimate returns the moment-scale diagnostic used by Moment.
It is not an exact Hawkes central moment; exact count moments require the observation window.
*/
func BranchingMomentEstimate(
	params BivariateParams, momentR, momentS float64,
) (float64, bool) {
	lambdaX, lambdaY, ok := params.MeanIntensity()

	if !ok {
		return 0, false
	}

	branching := params.branchingMatrix()

	switch {
	case momentR == 2 && momentS == 0:
		return lambdaX + 2*branching[0][0]*lambdaX, true
	case momentR == 0 && momentS == 2:
		return lambdaY + 2*branching[1][1]*lambdaY, true
	case momentR == 1 && momentS == 1:
		return branching[0][1]*lambdaY + branching[1][0]*lambdaX, true
	default:
		return 0, false
	}
}

/*
MomentConfidence returns a fit score in (0, 1] from empirical and estimated moments.
*/
func MomentConfidence(
	empirical, estimate float64,
) (float64, bool) {
	scale := math.Max(math.Abs(estimate), math.Abs(empirical))

	if scale <= 0 {
		return 0, false
	}

	residual := math.Abs(empirical-estimate) / scale

	return 1 / (1 + residual), true
}

/*
CrossAsymmetry compares third-order mixed moments between x and y streams.
*/
func CrossAsymmetry(x, y, weights []float64) (float64, bool) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, false
	}

	moment21 := stat.BivariateMoment(2, 1, x, y, weights)
	moment12 := stat.BivariateMoment(1, 2, x, y, weights)

	return moment21 - moment12, true
}
