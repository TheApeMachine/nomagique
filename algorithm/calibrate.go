package algorithm

import (
	"io"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/correlation"
	"github.com/theapemachine/nomagique/hawkes"
	"github.com/theapemachine/nomagique/learning"
	"github.com/theapemachine/nomagique/statistic"
)

/*
NewCalibrate returns an online RLS calibration stage.
*/
func NewCalibrate(dimension int, initialVariance float64) (io.ReadWriteCloser, error) {
	return learning.NewRLS(dimension, initialVariance)
}

/*
NewCorrelate returns a dual-correlation gap stage.
*/
func NewCorrelate(artifact *datura.Artifact) io.ReadWriteCloser {
	return correlation.NewGap(artifact)
}

/*
NewShift returns a distribution-shift KL divergence stage.
*/
func NewShift(expectedSum, floor float64) io.ReadWriteCloser {
	return statistic.NewKLDivergence(expectedSum, floor)
}

/*
NewHawkes returns a Hawkes moment-confidence stage.
*/
func NewHawkes(params hawkes.BivariateParams, momentR, momentS float64) io.ReadWriteCloser {
	return hawkes.NewMoment(params, momentR, momentS)
}

/*
NewHawkesFit returns a timestamp-stream Hawkes fit stage.
*/
func NewHawkesFit(horizonUnixNano float64, prior hawkes.BivariateFit) io.ReadWriteCloser {
	return hawkes.NewFit(int64(horizonUnixNano), prior)
}
