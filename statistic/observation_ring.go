package statistic

/*
ObservationRing retains recent scalar observations with capacity derived from
sample span rather than a fixed magic bound.
*/
type ObservationRing struct {
	samples []float64
}

func NewObservationRing() *ObservationRing {
	return &ObservationRing{}
}

func (ring *ObservationRing) Observe(value float64) {
	if value <= 0 {
		return
	}

	ring.samples = append(ring.samples, value)
	capacity := ring.capacityFor(ring.samples)

	if capacity <= 0 || len(ring.samples) <= capacity {
		return
	}

	ring.samples = ring.samples[len(ring.samples)-capacity:]
}

func (ring *ObservationRing) Len() int {
	return len(ring.samples)
}

func (ring *ObservationRing) Samples() []float64 {
	return ring.samples
}

func (ring *ObservationRing) Median() float64 {
	return MedianOf(ring.samples)
}

func (ring *ObservationRing) Quantile(percentile float64) float64 {
	return QuantileOf(percentile, ring.samples)
}

func (ring *ObservationRing) MedianAbsolute() float64 {
	return MedianAbsoluteOf(ring.samples)
}

func (ring *ObservationRing) Span() float64 {
	return SpanOf(ring.samples)
}

func (ring *ObservationRing) capacityFor(values []float64) int {
	if len(values) == 0 {
		return 1
	}

	if len(values) < 3 {
		return len(values) + 1
	}

	span := SpanOf(values)

	if span <= 0 {
		return len(values) + 1
	}

	capacity := int(span) + 1

	if capacity < len(values) {
		return len(values)
	}

	return capacity
}
