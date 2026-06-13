package algorithm

import (
	"math"

	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/learning"
)

/*
Trust combines forecast scale calibration with adaptive prediction trust.
*/
type Trust struct {
	stageParser *core.StageParser
	forecaster  *learning.Forecaster
	weight      *learning.TrustWeight
	lastTrust   float64
}

/*
NewTrust creates a calibration-trust dynamic over predicted-vs-actual pairs.
*/
func NewTrust() *Trust {
	return &Trust{
		stageParser: core.NewStageParser(),
		forecaster:  learning.Forecast(),
		weight:      learning.Weight(),
	}
}

/*
Observe ingests a predicted and actual pair and returns trust-weighted calibration.
*/
func (trust *Trust) Observe(inputs ...core.Number) core.Float64 {
	out, work, err := trust.stageParser.Parse(inputs)

	if err != nil {
		return 0
	}

	score, scoreErr := trust.Apply(out, work)

	if scoreErr != nil {
		return 0
	}

	return score
}

/*
Apply runs one pipeline stage without allocating number inputs.
*/
func (trust *Trust) Apply(
	out core.Float64, work []core.Float64,
) (core.Float64, error) {
	_, forecastErr := trust.forecaster.Apply(out, work)

	if forecastErr != nil {
		return 0, forecastErr
	}

	trustValue, weightErr := trust.weight.Apply(out, work)

	if weightErr != nil {
		return 0, weightErr
	}

	trust.lastTrust = float64(trustValue)

	scale := trust.forecaster.Scale()
	calibration := 1 - math.Abs(1-scale)

	if calibration < 0 {
		calibration = 0
	}

	return core.Float64(float64(trustValue) * calibration), nil
}

/*
Scale returns the current forecast scale.
*/
func (trust *Trust) Scale() float64 {
	return trust.forecaster.Scale()
}

/*
Weight returns the current adaptive trust weight.
*/
func (trust *Trust) Weight() core.Float64 {
	return core.Float64(trust.lastTrust)
}

/*
Reset clears derived state.
*/
func (trust *Trust) Reset() error {
	trust.lastTrust = 0

	if err := trust.forecaster.Reset(); err != nil {
		return err
	}

	return trust.weight.Reset()
}
