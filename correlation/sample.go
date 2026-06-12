package correlation

import (
	"time"

	"github.com/theapemachine/nomagique/core"
)

/*
Sample is a time-stamped observation for asynchronous correlation.
Each input pair encodes Unix seconds at an even index and value at the next index.
*/
type Sample struct {
	At    time.Time
	Value float64
}

func samplesFromNumbers(numbers core.Numbers) ([]Sample, bool) {
	values := numbers.Float64()

	if len(values) < 4 || len(values)%2 != 0 {
		return nil, false
	}

	samples := make([]Sample, len(values)/2)

	for index := range samples {
		pair := index * 2
		seconds := values[pair]
		wholeSeconds := int64(seconds)
		nanoseconds := int64((seconds - float64(wholeSeconds)) * float64(time.Second))

		samples[index] = Sample{
			At:    time.Unix(wholeSeconds, nanoseconds),
			Value: values[pair+1],
		}
	}

	return samples, true
}
