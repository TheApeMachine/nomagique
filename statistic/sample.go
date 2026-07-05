package statistic

import (
	"math"
	"time"

	"github.com/theapemachine/errnie"
)

/*
ScalarOutput reports the latest scalar statistic and whether enough history exists.
*/
type ScalarOutput struct {
	Value float64
	Ready bool
	Count int
}

/*
PairSample carries paired observations for bivariate statistics.
*/
type PairSample struct {
	Sample float64
	Paired float64
}

/*
PanelSample carries one cross-section member observation.
*/
type PanelSample struct {
	Member string
	Value  float64
}

/*
PanelOutput reports the retained cross-section after an observation.
*/
type PanelOutput struct {
	Peers map[string]float64
	Value float64
	Count int
}

/*
TimedSample carries one observation with event time and optional series key.
*/
type TimedSample struct {
	Series string
	Value  float64
	At     time.Time
}

func finiteStatistic(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			name+": value must be finite",
			nil,
		))
	}

	return nil
}

func finitePositiveStatistic(name string, value float64) error {
	if err := finiteStatistic(name, value); err != nil {
		return err
	}

	if value <= 0 {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			name+": value must be positive",
			nil,
		))
	}

	return nil
}
