package correlation

import (
	"math"

	"github.com/theapemachine/datura"
)

func pushSpread(artifact *datura.Artifact, capacity int, value float64) {
	if capacity <= 0 {
		capacity = 1
	}

	values := datura.Peek[[]float64](artifact, "spread", "values")
	head := int(datura.Peek[float64](artifact, "spread", "head"))
	count := int(datura.Peek[float64](artifact, "spread", "count"))

	if len(values) != capacity {
		values = make([]float64, capacity)
		head = 0
		count = 0
	}

	values[head] = value
	head = (head + 1) % capacity

	if count < capacity {
		count++
	}

	artifact.Poke(values, "spread", "values")
	artifact.Poke(float64(head), "spread", "head")
	artifact.Poke(float64(count), "spread", "count")
}

func spreadAt(artifact *datura.Artifact, index int) float64 {
	values := datura.Peek[[]float64](artifact, "spread", "values")
	head := int(datura.Peek[float64](artifact, "spread", "head"))
	count := int(datura.Peek[float64](artifact, "spread", "count"))
	capacity := len(values)

	if index < 0 || index >= count || capacity == 0 {
		return 0
	}

	start := 0

	if count >= capacity {
		start = head
	}

	return values[(start+index)%capacity]
}

func spreadLength(artifact *datura.Artifact) int {
	return int(datura.Peek[float64](artifact, "spread", "count"))
}

func adaptiveSpreadThreshold(
	artifact *datura.Artifact,
	slowBaseline float64,
	sigma float64,
) float64 {
	count := spreadLength(artifact)

	if count < 4 {
		if slowBaseline > 0 {
			return slowBaseline
		}

		return 0
	}

	mean := 0.0

	for index := 0; index < count; index++ {
		mean += spreadAt(artifact, index)
	}

	mean /= float64(count)

	if count < 2 {
		return mean
	}

	variance := 0.0

	for index := 0; index < count; index++ {
		delta := spreadAt(artifact, index) - mean
		variance += delta * delta
	}

	stddev := math.Sqrt(variance / float64(count-1))
	floor := mean * mean / (mean + slowBaseline)

	if stddev <= 0 {
		return math.Max(floor, mean)
	}

	return math.Max(floor, mean+sigma*stddev)
}
