package hawkes

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
Moment validates bivariate exponential-kernel parameters through empirical moments.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Moment struct {
	artifact *datura.Artifact
}

/*
NewMoment creates a Hawkes moment-confidence stage wired from config attributes.
momentR and momentS on the artifact select the mixed moment used for fit diagnostics.
*/
func NewMoment(artifact *datura.Artifact) *Moment {
	return &Moment{
		artifact: artifact,
	}
}

func (moment *Moment) Read(p []byte) (int, error) {
	state := datura.Acquire("hawkes-moment-state", datura.APPJSON)

	if _, err := state.Unpack(moment.artifact.DecryptPayload()); err != nil {
		state.Release()
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes-moment: state write failed",
			err,
		))
	}

	xValues, yValues, weights, ok := momentSamples(state, moment.artifact)

	if !ok {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes-moment: require aligned sample streams of at least two observations",
			nil,
		))
	}

	params := bivariateParamsFromArtifact(moment.artifact)
	momentR := datura.Peek[float64](moment.artifact, "config", "momentR")
	momentS := datura.Peek[float64](moment.artifact, "config", "momentS")

	if momentR < 0 || momentS < 0 || momentR+momentS == 0 {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes-moment: config momentR and momentS must define a nonzero moment",
			nil,
		))
	}

	empirical := stat.BivariateMoment(momentR, momentS, xValues, yValues, weights)
	estimate, estimateOK := BranchingMomentEstimate(params, momentR, momentS)

	if !estimateOK {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes-moment: branching moment estimate unavailable for parameters",
			nil,
		))
	}

	confidence, confidenceOK := MomentConfidence(empirical, estimate)

	if !confidenceOK {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes-moment: confidence could not be derived",
			nil,
		))
	}

	state.MergeOutput("value", confidence)
	state.MergeOutput("empirical", empirical)
	state.MergeOutput("estimate", estimate)
	state.MergeOutput("confidence", confidence)
	state.Poke("output", "root")
	state.Poke([]string{"value", "empirical", "estimate", "confidence"}, "inputs")

	return state.PackInto(p)
}

func (moment *Moment) Write(p []byte) (int, error) {
	moment.artifact.WithPayload(p)
	return len(p), nil
}

func (moment *Moment) Close() error {
	return nil
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
