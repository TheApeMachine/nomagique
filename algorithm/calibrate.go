package algorithm

import (
	"math"

	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/learning"
)

/*
Calibrate fits an online RLS model from aligned feature and target streams.
*/
type Calibrate[T ~float64] struct {
	features         [][]float64
	target           []float64
	filter           *learning.RLSFilter
	initialVariance  float64
	forgettingFactor float64
	lastResidual     float64
	lastCondition    float64
	output           core.Scalar[T]
}

/*
NewCalibrate creates an RLS calibration dynamic over feature streams and one target stream.
initialVariance and forgettingFactor are forwarded to learning.RLSFilter.
*/
func NewCalibrate[T ~float64](
	features [][]float64,
	target []float64,
	initialVariance, forgettingFactor float64,
) (*Calibrate[T], error) {
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

	return &Calibrate[T]{
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
func (calibrate *Calibrate[T]) Observe(_ ...core.Number[T]) core.Scalar[T] {
	rows, ok := zipFeatureTargetRows(calibrate.features, calibrate.target)

	if !ok {
		calibrate.lastResidual = 0
		calibrate.lastCondition = 0
		calibrate.output = core.Scalar[T](0)

		return calibrate.output
	}

	calibrate.filter.Reset()

	for rowIndex, row := range rows {
		observeErr := calibrate.filter.Observe(row.features, row.target)

		if observeErr != nil {
			calibrate.lastResidual = 0
			calibrate.lastCondition = 0
			calibrate.output = core.Scalar[T](0)

			return calibrate.output
		}

		if rowIndex == len(rows)-1 {
			residual, residualErr := calibrate.filter.Residual(row.features, row.target)

			if residualErr != nil {
				calibrate.lastResidual = 0
				calibrate.lastCondition = calibrate.filter.ConditionNumberBound()
				calibrate.output = core.Scalar[T](0)

				return calibrate.output
			}

			calibrate.lastResidual = math.Abs(residual)
			calibrate.lastCondition = calibrate.filter.ConditionNumberBound()
			calibrate.output = core.Scalar[T](T(calibrate.lastResidual))

			return calibrate.output
		}
	}

	calibrate.output = core.Scalar[T](0)

	return calibrate.output
}

/*
Residual returns the absolute residual from the last Observe call.
*/
func (calibrate *Calibrate[T]) Residual() core.Scalar[T] {
	return core.Scalar[T](T(calibrate.lastResidual))
}

/*
ConditionNumber returns the covariance condition bound from the last Observe call.
*/
func (calibrate *Calibrate[T]) ConditionNumber() core.Scalar[T] {
	return core.Scalar[T](T(calibrate.lastCondition))
}

/*
Coefficients returns a copy of the current RLS coefficients including intercept.
*/
func (calibrate *Calibrate[T]) Coefficients() []float64 {
	return calibrate.filter.Coefficients()
}

/*
Reset clears derived state and restores the RLS filter.
*/
func (calibrate *Calibrate[T]) Reset() error {
	calibrate.lastResidual = 0
	calibrate.lastCondition = 0
	calibrate.output = core.Scalar[T](0)
	calibrate.filter.Reset()

	return nil
}

type featureTargetRow struct {
	features []float64
	target   float64
}

func zipFeatureTargetRows(
	features [][]float64, target []float64,
) ([]featureTargetRow, bool) {
	if len(features) == 0 {
		return nil, false
	}

	rowCount := len(features[0])

	if rowCount == 0 || len(target) != rowCount {
		return nil, false
	}

	rows := make([]featureTargetRow, rowCount)

	for rowIndex := range rows {
		rowFeatures := make([]float64, len(features))

		for featureIndex, featureStream := range features {
			if len(featureStream) != rowCount {
				return nil, false
			}

			rowFeatures[featureIndex] = featureStream[rowIndex]
		}

		rows[rowIndex] = featureTargetRow{
			features: rowFeatures,
			target:   target[rowIndex],
		}
	}

	return rows, true
}
