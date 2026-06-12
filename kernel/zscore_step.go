package kernel

/*
ObserveZScore ingests one sample and returns a z-score.
When hasAnchorMean is true, anchorMean is the level and only variance is adapted.
*/
func ObserveZScore(
	state *ZScoreState, sample float64, anchorMean float64, hasAnchorMean bool,
) float64 {
	if !state.Ready {
		state.Mean = sample
		state.Var = 0
		state.Prev = sample
		state.Min = sample
		state.Max = sample
		state.Ready = true
		return 0
	}

	return observeZScoreReady(state, sample, anchorMean, hasAnchorMean)
}

/*
observeZScoreReady runs the hot z-score path; state must already be Ready.
*/
func observeZScoreReady(
	state *ZScoreState, sample float64, anchorMean float64, hasAnchorMean bool,
) float64 {
	if sample < state.Min {
		state.Min = sample
	}

	if sample > state.Max {
		state.Max = sample
	}

	span := state.Max - state.Min

	if span == 0 {
		state.Prev = sample
		return 0
	}

	delta := absExact(sample - state.Prev)
	state.Rate = delta / span
	level := state.Mean

	if hasAnchorMean {
		level = anchorMean
	}

	deviation := sample - level

	if !hasAnchorMean {
		state.Mean += state.Rate * (sample - state.Mean)
	}

	state.Var += state.Rate * (deviation*deviation - state.Var)
	state.Prev = sample

	return zScoreFromDeviation(deviation, state.Var)
}
