package statistic

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
BivariateMoment computes E[(x - mu_x)^r (y - mu_y)^s] for two configured streams.
*/
type BivariateMoment struct {
	artifact *datura.Artifact
	r        float64
	s        float64
	weights  []float64
}

/*
NewBivariateMoment creates a bivariate moment stage at exponents r and s.
Payload carries aligned x then y samples as big-endian float64 values.
*/
func NewBivariateMoment(
	r, s float64,
	weights []float64,
) *BivariateMoment {
	return &BivariateMoment{
		artifact: datura.Acquire("bivariate_moment", datura.Artifact_Type_json),
		r:        r,
		s:        s,
		weights:  weights,
	}
}

/*
Powers returns the mixed-moment exponents r and s.
*/
func (bivariateMoment *BivariateMoment) Powers() (r float64, s float64) {
	return bivariateMoment.r, bivariateMoment.s
}

func (bivariateMoment *BivariateMoment) Write(p []byte) (int, error) {
	return bivariateMoment.artifact.Write(p)
}

func (bivariateMoment *BivariateMoment) Read(p []byte) (int, error) {
	payload, err := bivariateMoment.artifact.Payload()

	if err != nil || len(payload) < 16 || len(payload)%8 != 0 {
		return bivariateMoment.artifact.Read(p)
	}

	count := len(payload) / 8

	if count%2 != 0 {
		errnie.Err(
			errnie.Validation, "unable to compute bivariate moment",
			BivariateMomentError(BivariateMomentErrorInvalidStreams),
		)

		putFloat64Payload(&bivariateMoment.artifact, "bivariate_moment", 0)

		return bivariateMoment.artifact.Read(p)
	}

	half := count / 2
	xValues := make([]float64, half)
	yValues := make([]float64, half)

	for index := range half {
		xOffset := index * 8
		yOffset := (index + half) * 8
		xValues[index] = math.Float64frombits(binary.BigEndian.Uint64(payload[xOffset : xOffset+8]))
		yValues[index] = math.Float64frombits(binary.BigEndian.Uint64(payload[yOffset : yOffset+8]))
	}

	weights := bivariateMoment.weights

	if len(weights) == 0 {
		weights = nil
	}

	if len(xValues) != len(yValues) || len(xValues) < 2 {
		errnie.Err(
			errnie.Validation, "unable to compute bivariate moment",
			BivariateMomentError(BivariateMomentErrorInvalidStreams),
		)

		putFloat64Payload(&bivariateMoment.artifact, "bivariate_moment", 0)

		return bivariateMoment.artifact.Read(p)
	}

	if len(weights) != 0 && len(weights) != len(xValues) {
		errnie.Err(
			errnie.Validation, "unable to compute bivariate moment",
			BivariateMomentError(BivariateMomentErrorWeightLengthMismatch),
		)

		putFloat64Payload(&bivariateMoment.artifact, "bivariate_moment", 0)

		return bivariateMoment.artifact.Read(p)
	}

	moment := stat.BivariateMoment(
		bivariateMoment.r, bivariateMoment.s,
		xValues, yValues, weights,
	)

	putFloat64Payload(&bivariateMoment.artifact, "bivariateMoment", moment)

	return bivariateMoment.artifact.Read(p)
}

func (bivariateMoment *BivariateMoment) Close() error {
	return nil
}

func (bivariateMoment *BivariateMoment) Reset() error {
	bivariateMoment.weights = nil

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
