package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
BivariateMoment computes E[(x - mu_x)^r (y - mu_y)^s] for paired sample streams.
*/
type BivariateMoment struct {
	artifact *datura.Artifact
	r        float64
	s        float64
}

/*
NewBivariateMoment creates a bivariate moment stage at exponents r and s.
*/
func NewBivariateMoment(r, s float64) *BivariateMoment {
	return &BivariateMoment{
		artifact: datura.Acquire("bivariate_moment", datura.APPJSON),
		r:        r,
		s:        s,
	}
}

func (bivariateMoment *BivariateMoment) Write(p []byte) (int, error) {
	return bivariateMoment.artifact.Write(p)
}

func (bivariateMoment *BivariateMoment) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](bivariateMoment.artifact, "sample")
	paired := datura.Peek[float64](bivariateMoment.artifact, "paired")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return bivariateMoment.artifact.Read(p)
	}

	if math.IsNaN(paired) || math.IsInf(paired, 0) {
		return bivariateMoment.artifact.Read(p)
	}

	xValues := datura.Peek[[]float64](bivariateMoment.artifact, "history")
	xValues = append(xValues, sample)
	bivariateMoment.artifact.Poke(xValues, "history")

	yValues := datura.Peek[[]float64](bivariateMoment.artifact, "pairedHistory")
	yValues = append(yValues, paired)
	bivariateMoment.artifact.Poke(yValues, "pairedHistory")

	if len(xValues) != len(yValues) || len(xValues) < 2 {
		errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute bivariate moment",
			BivariateMomentError(BivariateMomentErrorInvalidStreams),
		))

		bivariateMoment.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return bivariateMoment.artifact.Read(p)
	}

	moment := stat.BivariateMoment(
		bivariateMoment.r, bivariateMoment.s,
		xValues, yValues, nil,
	)

	bivariateMoment.artifact.Poke(datura.Map[float64]{"value": moment}, "output")

	return bivariateMoment.artifact.Read(p)
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
