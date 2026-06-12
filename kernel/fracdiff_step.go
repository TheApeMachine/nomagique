package kernel

/*
ObserveFracDiff ingests one sample into state and returns the filtered value.
*/
func ObserveFracDiff(state *FracDiffState, sample float64) float64 {
	if !state.Ready {
		state.Min = sample
		state.Max = sample
		state.Prev = sample
		state.Order = 0
		state.Ready = true
		state.Width = 1
		state.Head = 0
		state.Count = 1
		state.History = make([]float64, fracDiffMaxLag(0)+1)
		state.History[0] = sample
		state.Weights = []float64{1}

		return sample
	}

	return observeFracDiffReady(state, sample)
}

/*
observeFracDiffReady runs the hot fractional differencing path.
*/
func observeFracDiffReady(state *FracDiffState, sample float64) float64 {
	if sample < state.Min {
		state.Min = sample
	}

	if sample > state.Max {
		state.Max = sample
	}

	span := state.Max - state.Min

	if span == 0 {
		state.pushHistory(sample)
		state.Prev = sample

		return sample
	}

	rate := absExact(sample-state.Prev) / span
	order := fracDiffOrder(rate, span)
	state.maybeRebuildWeights(order, span)
	state.pushHistory(sample)
	state.Prev = sample

	return fracDiffOutput(state)
}

func (state *FracDiffState) maybeRebuildWeights(order float64, span float64) {
	if order == state.Order && state.Width > 0 {
		return
	}

	state.Order = order

	capacity := fracDiffMaxLag(span) + 1

	if cap(state.Weights) < capacity {
		state.Weights = make([]float64, 0, capacity)
	}

	weights, width := buildFracDiffWeights(order, span, state.Prev, state.Weights[:0])
	state.Weights = weights[:width]
	state.Width = width
	state.ensureHistoryCapacity(capacity)
}

func (state *FracDiffState) ensureHistoryCapacity(capacity int) {
	if len(state.History) >= capacity {
		return
	}

	next := make([]float64, capacity)
	copy(next, state.History)

	if state.Count > 0 {
		for index := 0; index < state.Count; index++ {
			source := (state.Head - index + len(state.History)) % len(state.History)
			next[index] = state.History[source]
		}

		state.Head = state.Count - 1
	}

	state.History = next
}

func (state *FracDiffState) pushHistory(sample float64) {
	if len(state.History) == 0 {
		return
	}

	state.Head = (state.Head + 1) % len(state.History)
	state.History[state.Head] = sample

	if state.Count < len(state.History) {
		state.Count++
	}
}

func fracDiffOutput(state *FracDiffState) float64 {
	sum := 0.0
	limit := state.Width

	if state.Count < limit {
		limit = state.Count
	}

	for lag := 0; lag < limit; lag++ {
		index := state.Head - lag

		if index < 0 {
			index += len(state.History)
		}

		sum += state.Weights[lag] * state.History[index]
	}

	return sum
}
