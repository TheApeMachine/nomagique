package algorithm

import "github.com/theapemachine/nomagique/learning"

/*
TrustOutput reports combined trust and forecast calibration.
*/
type TrustOutput struct {
	Value    float64
	Trust    learning.TrustWeightOutput
	Forecast learning.ForecastOutput
}

/*
Trust composes trust-weight and forecast learners over predicted-vs-actual pairs.
*/
type Trust struct {
	weight   *learning.TrustWeight
	forecast *learning.Forecaster
}

/*
NewTrust returns a typed calibration-trust learner.
*/
func NewTrust() *Trust {
	return &Trust{
		weight:   learning.NewTrustWeight(),
		forecast: learning.Forecast(),
	}
}

/*
Measure updates both trust learners and returns their combined calibration.
*/
func (trust *Trust) Measure(pair learning.LearningPair) (TrustOutput, error) {
	weight, err := trust.weight.Measure(pair)
	if err != nil {
		return TrustOutput{}, err
	}

	forecast, err := trust.forecast.Measure(pair)
	if err != nil {
		return TrustOutput{}, err
	}

	return TrustOutput{
		Value:    weight.Value * forecast.Value,
		Trust:    weight,
		Forecast: forecast,
	}, nil
}
