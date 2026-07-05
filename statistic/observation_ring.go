package statistic

import "github.com/theapemachine/errnie"

/*
ObservationRingConfig configures a bounded positive-observation ring.
*/
type ObservationRingConfig struct {
	Capacity int
}

/*
ObservationRing retains a bounded history of positive observations.
*/
type ObservationRing struct {
	capacity int
	history  []float64
}

/*
NewObservationRing returns a typed observation ring.
*/
func NewObservationRing(config ObservationRingConfig) *ObservationRing {
	return &ObservationRing{
		capacity: config.Capacity,
	}
}

/*
Measure adds one positive sample and returns the sample when retained.
*/
func (observationRing *ObservationRing) Measure(sample float64) (ScalarOutput, error) {
	if observationRing.capacity <= 0 {
		return ScalarOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"observation-ring: capacity required",
			nil,
		))
	}

	if err := finitePositiveStatistic("observation-ring", sample); err != nil {
		return ScalarOutput{}, err
	}

	observationRing.history = append(observationRing.history, sample)

	if len(observationRing.history) > observationRing.capacity {
		observationRing.history = observationRing.history[len(observationRing.history)-observationRing.capacity:]
	}

	return ScalarOutput{
		Value: sample,
		Ready: true,
		Count: len(observationRing.history),
	}, nil
}

/*
History returns the retained observations.
*/
func (observationRing *ObservationRing) History() []float64 {
	return append([]float64(nil), observationRing.history...)
}
