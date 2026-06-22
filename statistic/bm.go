package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
BivariateMoment computes E[(x - mu_x)^r (y - mu_y)^s] for paired sample streams.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type BivariateMoment struct {
	artifact *datura.Artifact
}

/*
NewBivariateMoment returns a bivariate moment stage wired from config attributes on the artifact.
Exponents live under config.r and config.s.
*/
func NewBivariateMoment(artifact *datura.Artifact) *BivariateMoment {
	artifact.Inspect("statistic", "bivariate-moment", "NewBivariateMoment()")

	return &BivariateMoment{
		artifact: artifact,
	}
}

func (bivariateMoment *BivariateMoment) Write(payload []byte) (int, error) {
	bivariateMoment.artifact.WithPayload(payload)
	return len(payload), nil
}

func (bivariateMoment *BivariateMoment) Read(payload []byte) (int, error) {
	state := datura.Acquire("bivariate-moment-state", datura.APPJSON)
	state.Inspect("statistic", "bivariate-moment", "Read()", "p")

	if _, err := state.Write(bivariateMoment.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sample := datura.Peek[float64](state, "sample")
	paired := datura.Peek[float64](state, "paired")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"unable to compute bivariate moment",
			BivariateMomentError(BivariateMomentErrorInvalidStreams),
		))
	}

	if math.IsNaN(paired) || math.IsInf(paired, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"unable to compute bivariate moment",
			BivariateMomentError(BivariateMomentErrorInvalidStreams),
		))
	}

	xValues := datura.Peek[[]float64](bivariateMoment.artifact, "history")
	xValues = append(xValues, sample)
	bivariateMoment.artifact.Poke(xValues, "history")

	yValues := datura.Peek[[]float64](bivariateMoment.artifact, "pairedHistory")
	yValues = append(yValues, paired)
	bivariateMoment.artifact.Poke(yValues, "pairedHistory")

	if len(xValues) != len(yValues) || len(xValues) < 2 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"unable to compute bivariate moment",
			BivariateMomentError(BivariateMomentErrorInvalidStreams),
		))
	}

	exponentR := datura.Peek[float64](bivariateMoment.artifact, "config", "r")
	exponentS := datura.Peek[float64](bivariateMoment.artifact, "config", "s")
	moment := stat.BivariateMoment(
		exponentR, exponentS,
		xValues, yValues, nil,
	)

	state.MergeOutput("value", moment)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (bivariateMoment *BivariateMoment) Close() error {
	return nil
}

type BivariateMomentErrorType string

const (
	BivariateMomentErrorInvalidStreams       BivariateMomentErrorType = "require aligned streams of at least two samples"
	BivariateMomentErrorWeightLengthMismatch BivariateMomentErrorType = "require equal weight length"
)

type BivariateMomentError string

func (bivariateMomentError BivariateMomentError) Error() string {
	return string(bivariateMomentError)
}
