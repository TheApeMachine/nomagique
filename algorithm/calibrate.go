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
NewCalibrate returns an online RLS calibration stage wired from config on the artifact.
*/
func NewCalibrate(artifact *datura.Artifact) io.ReadWriteCloser {
	return learning.NewRLS(artifact)
}

/*
NewCorrelate returns a dual-correlation gap stage.
*/
func NewCorrelate(artifact *datura.Artifact) io.ReadWriteCloser {
	return correlation.NewGap(artifact)
}

/*
NewShift returns a distribution-shift KL divergence stage wired from config on the artifact.
*/
func NewShift(artifact *datura.Artifact) io.ReadWriteCloser {
	return statistic.NewKLDivergence(artifact)
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
