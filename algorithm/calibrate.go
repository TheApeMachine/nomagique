package algorithm

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/learning"
)

/*
Calibrate fits an online RLS model from aligned feature and target streams.
*/
type Calibrate struct {
	artifact         *datura.Artifact
	features         [][]float64
	target           []float64
	filter           *learning.RLSFilter
	initialVariance  float64
	forgettingFactor float64
	lastResidual     float64
	lastCondition    float64
}

/*
NewCalibrate creates an RLS calibration stage over feature streams and one target stream.
initialVariance and forgettingFactor are forwarded to learning.RLSFilter.
*/
func NewCalibrate(
	features [][]float64,
	target []float64,
	initialVariance, forgettingFactor float64,
) (*Calibrate, error) {
	if len(features) == 0 {
		return nil, ErrEmptyInputs
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
		artifact:         datura.Acquire("calibrate", datura.Artifact_Type_json),
		features:         features,
		target:           target,
		filter:           filter,
		initialVariance:  initialVariance,
		forgettingFactor: forgettingFactor,
	}, nil
}

func (calibrate *Calibrate) Write(p []byte) (int, error) {
	return calibrate.artifact.Write(p)
}

func (calibrate *Calibrate) Read(p []byte) (int, error) {
	rehydrateArtifact(&calibrate.artifact, "calibrate", datura.Artifact_Type_json)

	rows, ok := zipFeatureTargetRows(calibrate.features, calibrate.target)

	if !ok {
		calibrate.lastResidual = 0
		calibrate.lastCondition = 0

		return calibrate.artifact.Read(p)
	}

	calibrate.filter.Reset()

	for rowIndex, row := range rows {
		observeErr := calibrate.filter.Observe(row.features, row.target)

		if observeErr != nil {
			calibrate.lastResidual = 0
			calibrate.lastCondition = 0

			return calibrate.artifact.Read(p)
		}

		if rowIndex == len(rows)-1 {
			residual, residualErr := calibrate.filter.Residual(row.features, row.target)

			if residualErr != nil {
				calibrate.lastResidual = 0
				calibrate.lastCondition = calibrate.filter.ConditionNumberBound()

				return calibrate.artifact.Read(p)
			}

			calibrate.lastResidual = math.Abs(residual)
			calibrate.lastCondition = calibrate.filter.ConditionNumberBound()
			out := encodePayload(calibrate.lastResidual)
			_ = calibrate.artifact.SetPayload(out)

			return calibrate.artifact.Read(p)
		}
	}

	return calibrate.artifact.Read(p)
}

func (calibrate *Calibrate) Close() error {
	return nil
}

/*
Residual returns the absolute residual from the last Read call.
*/
func (calibrate *Calibrate) Residual() float64 {
	return calibrate.lastResidual
}

/*
ConditionNumber returns the covariance condition bound from the last Read call.
*/
func (calibrate *Calibrate) ConditionNumber() float64 {
	return calibrate.lastCondition
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
