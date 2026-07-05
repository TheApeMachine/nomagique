package algorithm

import (
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
NewShift returns a typed distribution-shift KL divergence stage.
*/
func NewShift() *statistic.KLDivergence {
	return statistic.NewKLDivergence()
}
