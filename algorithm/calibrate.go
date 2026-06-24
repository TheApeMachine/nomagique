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
	config := datura.Acquire("shift-config", datura.APPJSON).
		Poke(expectedSum, "config", "expectedSum").
		Poke(floor, "config", "floor").
		Poke("sample", "sampleKey").
		Poke("paired", "pairedKey").
		Poke("value", "outputKey")

	return statistic.NewKLDivergence(config)
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
