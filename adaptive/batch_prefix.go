package adaptive

const vectorBatchThreshold = 32

/*
prefixMinMax writes running min and max after each sample, seeded from initMin and initMax.
Associative prefix form; SIMD drivers implement the same recurrence in wider steps.
*/
func prefixMinMax(
	initMin float64, initMax float64,
	samples []float64, minOut []float64, maxOut []float64,
) {
	runningMin := initMin
	runningMax := initMax

	for index, sample := range samples {
		if sample < runningMin {
			runningMin = sample
		}

		if sample > runningMax {
			runningMax = sample
		}

		minOut[index] = runningMin
		maxOut[index] = runningMax
	}
}

func prefixMinMaxVector(
	initMin float64, initMax float64,
	samples []float64, minOut []float64, maxOut []float64,
) {
	prefixMinMax(initMin, initMax, samples, minOut, maxOut)
}
