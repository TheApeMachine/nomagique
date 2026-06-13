package adaptive

/*
observeEMASamplesHotInlined is the batch driver with the ready-path body inlined for compiler fusion.
*/
func observeEMASamplesHotInlined(state *EMAState, samples []float64, out []float64) {
	value := state.Value
	prev := state.Prev
	min := state.Min
	max := state.Max
	rate := state.Rate

	for index, sample := range samples {
		if sample < min {
			min = sample
		}

		if sample > max {
			max = sample
		}

		span := max - min

		if span == 0 {
			prev = sample
			out[index] = value
			continue
		}

		delta := sample - prev

		if delta < 0 {
			delta = -delta
		}

		rate = delta / span
		value += rate * (sample - value)
		prev = sample
		out[index] = value
	}

	state.Value = value
	state.Prev = prev
	state.Min = min
	state.Max = max
	state.Rate = rate
}
