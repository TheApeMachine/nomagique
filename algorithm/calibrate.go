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
NewCalibrate returns an online typed RLS calibration learner.
*/
func NewCalibrate(config learning.RLSConfig) (*learning.RLS, error) {
	return learning.NewRLS(config)
}

/*
NewCorrelate returns a dual-correlation gap stage.
*/
func NewCorrelate(artifact *datura.Artifact) io.ReadWriteCloser {
	return correlation.NewGap(artifact)
}

/*
NewShift returns a typed distribution-shift KL divergence stage.
*/
func NewShift() *statistic.KLDivergence {
	return statistic.NewKLDivergence()
}

/*
NewHawkes returns a Hawkes moment-confidence stage wired from config on the artifact.
*/
func NewHawkes(artifact *datura.Artifact) io.ReadWriteCloser {
	return hawkes.NewMoment(artifact)
}

/*
NewHawkesFit returns a timestamp-stream Hawkes fit stage wired from config on the artifact.
*/
func NewHawkesFit(artifact *datura.Artifact) io.ReadWriteCloser {
	return hawkes.NewFit(artifact)
}
