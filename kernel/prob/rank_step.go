package prob

/*
ObserveRank ingests one sample and returns empirical P(history <= sample).
*/
func ObserveRank(state *RankState, sample float64) float64 {
	if !state.Ready {
		state.Min = sample
		state.Max = sample
		state.Prev = sample
		state.Ready = true
		state.History = make([]float64, rankCapacity(0)+1)
		state.History[0] = sample
		state.Head = 0
		state.Count = 1
		return 1
	}

	return observeRankReady(state, sample)
}

/*
observeRankReady runs the hot rank path; state must already be Ready.
*/
func observeRankReady(state *RankState, sample float64) float64 {
	if sample < state.Min {
		state.Min = sample
	}

	if sample > state.Max {
		state.Max = sample
	}

	span := state.Max - state.Min
	state.ensureCapacity(rankCapacity(span))
	state.pushHistory(sample)
	state.Prev = sample

	return empiricalRank(state, sample)
}

func rankCapacity(span float64) int {
	if span < 1 {
		return 1
	}

	return int(span) + 1
}

func (state *RankState) ensureCapacity(capacity int) {
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

func (state *RankState) pushHistory(sample float64) {
	if len(state.History) == 0 {
		return
	}

	state.Head = (state.Head + 1) % len(state.History)
	state.History[state.Head] = sample

	if state.Count < len(state.History) {
		state.Count++
	}
}

func empiricalRank(state *RankState, sample float64) float64 {
	if state.Count == 0 {
		return 0
	}

	atOrBelow := 0

	for index := 0; index < state.Count; index++ {
		historyIndex := state.Head - index

		if historyIndex < 0 {
			historyIndex += len(state.History)
		}

		if state.History[historyIndex] <= sample {
			atOrBelow++
		}
	}

	return float64(atOrBelow) / float64(state.Count)
}
