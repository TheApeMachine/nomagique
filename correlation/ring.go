package correlation

import "time"

/*
SampleRing is a fixed-capacity rolling window of timestamped samples.
*/
type SampleRing struct {
	samples []Sample
	head    int
	count   int
}

/*
NewSampleRing allocates one rolling window with the given capacity.
*/
func NewSampleRing(capacity int) SampleRing {
	if capacity <= 0 {
		capacity = 1
	}

	return SampleRing{samples: make([]Sample, capacity)}
}

/*
Push records one sample when the timestamp and value are valid.
*/
func (sampleRing *SampleRing) Push(at time.Time, value float64) {
	if at.IsZero() || value <= 0 {
		return
	}

	capacity := len(sampleRing.samples)
	sampleRing.samples[sampleRing.head] = Sample{At: at, Value: value}
	sampleRing.head = (sampleRing.head + 1) % capacity

	if sampleRing.count < capacity {
		sampleRing.count++
	}
}

/*
AppendOrdered appends the window contents from oldest to newest into destination.
*/
func (sampleRing SampleRing) AppendOrdered(destination []Sample) []Sample {
	if sampleRing.count == 0 {
		return destination[:0]
	}

	if cap(destination) < sampleRing.count {
		destination = make([]Sample, 0, sampleRing.count)
	}

	ordered := destination[:0]
	start := sampleRing.startIndex()

	for index := 0; index < sampleRing.count; index++ {
		ordered = append(
			ordered, sampleRing.samples[(start+index)%len(sampleRing.samples)],
		)
	}

	return ordered
}

func (sampleRing SampleRing) startIndex() int {
	if sampleRing.count < len(sampleRing.samples) {
		return 0
	}

	return sampleRing.head
}
