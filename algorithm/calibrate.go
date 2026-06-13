package algorithm

import (
	"math"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/learning"
)

/*
Calibrate fits an online RLS model from aligned feature and target streams.
*/
type Calibrate struct {
	features         []core.Numbers
	target           core.Numbers
	filter           *learning.RLSFilter
	initialVariance  float64
	forgettingFactor float64
	lastResidual     float64
	lastCondition    float64
}

/*
NewCalibrate creates an RLS calibration dynamic over feature streams and one target stream.
initialVariance and forgettingFactor are forwarded to learning.RLSFilter.
*/
func NewCalibrate(
	features []core.Numbers,
	target core.Numbers,
	initialVariance, forgettingFactor float64,
) (*Calibrate, error) {
	if len(features) == 0 {
		return nil, core.ErrEmptyInputs
	}

	if initialVariance <= 0 {
		initialVariance = 1000
	}

	if forgettingFactor <= 0 || forgettingFactor > 1 {
		forgettingFactor = 1
	}

	filter, err := learning.NewRLSFilter(len(features), initialVariance)

	if err != nil {
		return nil, err
	}

	if setErr := filter.SetForgettingFactor(forgettingFactor); setErr != nil {
		return nil, setErr
	}

	return &Calibrate{
		features:         features,
		target:           target,
		filter:           filter,
		initialVariance:  initialVariance,
		forgettingFactor: forgettingFactor,
	}, nil
}

/*
Observe replays stream history through the RLS filter and returns the last absolute residual.
*/
func (calibrate *Calibrate) Observe(_ ...core.Number) core.Float64 {
	rows, ok := zipFeatureTargetRows(calibrate.features, calibrate.target)

	if !ok {
		calibrate.lastResidual = 0
		calibrate.lastCondition = 0

		return 0
	}

	calibrate.filter.Reset()

	for rowIndex, row := range rows {
		observeErr := calibrate.filter.Observe(row.features, row.target)

		if observeErr != nil {
			calibrate.lastResidual = 0
			calibrate.lastCondition = 0

			return 0
		}

		if rowIndex == len(rows)-1 {
			residual, residualErr := calibrate.filter.Residual(row.features, row.target)

			if residualErr != nil {
				calibrate.lastResidual = 0
				calibrate.lastCondition = calibrate.filter.ConditionNumberBound()

				return 0
			}

			calibrate.lastResidual = math.Abs(residual)
			calibrate.lastCondition = calibrate.filter.ConditionNumberBound()

			return core.Float64(calibrate.lastResidual)
		}
	}

	return 0
}

/*
Residual returns the absolute residual from the last Observe call.
*/
func (calibrate *Calibrate) Residual() core.Float64 {
	return core.Float64(calibrate.lastResidual)
}

/*
ConditionNumber returns the covariance condition bound from the last Observe call.
*/
func (calibrate *Calibrate) ConditionNumber() core.Float64 {
	return core.Float64(calibrate.lastCondition)
}

/*
Coefficients returns a copy of the current RLS coefficients including intercept.
*/
func (calibrate *Calibrate) Coefficients() []float64 {
	return calibrate.filter.Coefficients()
}

/*
Reset clears derived state and restores the RLS filter.
*/
func (calibrate *Calibrate) Reset() error {
	calibrate.lastResidual = 0
	calibrate.lastCondition = 0
	calibrate.filter.Reset()

	return nil
}

type featureTargetRow struct {
	features []float64
	target   float64
}

func zipFeatureTargetRows(
	features []core.Numbers, target core.Numbers,
) ([]featureTargetRow, bool) {
	if len(features) == 0 {
		return nil, false
	}

	first := nomagique.Samples(features[0])
	rowCount := len(first)
	targetValues := nomagique.Samples(target)

	if rowCount == 0 || len(targetValues) != rowCount {
		return nil, false
	}

	rows := make([]featureTargetRow, rowCount)

	for rowIndex := range rows {
		rowFeatures := make([]float64, len(features))

		for featureIndex, featureStream := range features {
			samples := nomagique.Samples(featureStream)

			if len(samples) != rowCount {
				return nil, false
			}

			rowFeatures[featureIndex] = samples[rowIndex]
		}

		rows[rowIndex] = featureTargetRow{
			features: rowFeatures,
			target:   targetValues[rowIndex],
		}
	}

	return rows, true
}
