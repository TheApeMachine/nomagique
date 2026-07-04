package algorithm

/*
ExcitationReading exposes one ExcitationOutcome field as a score source.
*/
type ExcitationReading struct {
	project func(ExcitationOutcome) float64
}

func newExcitationReading(
	project func(ExcitationOutcome) float64,
) *ExcitationReading {
	return &ExcitationReading{
		project: project,
	}
}

func (excitation *Excitation) FrenzyReading() *ExcitationReading {
	return newExcitationReading(func(outcome ExcitationOutcome) float64 {
		return outcome.Frenzy
	})
}

func (excitation *Excitation) SaturationReading() *ExcitationReading {
	return newExcitationReading(func(outcome ExcitationOutcome) float64 {
		return outcome.Saturation
	})
}

func (excitation *Excitation) OrganicReading() *ExcitationReading {
	return newExcitationReading(func(outcome ExcitationOutcome) float64 {
		return outcome.Organic
	})
}

func (excitation *Excitation) ExhaustionReading() *ExcitationReading {
	return newExcitationReading(func(outcome ExcitationOutcome) float64 {
		return outcome.Exhaustion
	})
}

/*
Measure projects one score from an excitation outcome.
*/
func (reading *ExcitationReading) Measure(outcome ExcitationOutcome) float64 {
	if reading.project == nil {
		return 0
	}

	return reading.project(outcome)
}
