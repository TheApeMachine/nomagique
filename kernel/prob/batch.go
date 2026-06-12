package prob

func observeBetaSamples(state *BetaState, outcomes []float64, out []float64) {
	for index, outcome := range outcomes {
		out[index] = ObserveBeta(state, outcome)
	}
}

func observeBetaPairSamples(
	state *BetaState, predicted []float64, actual []float64, out []float64,
) {
	for index, predict := range predicted {
		out[index] = ObserveBetaPair(state, predict, actual[index])
	}
}

func observeCUSUMSamples(state *CUSUMState, samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = ObserveCUSUM(state, sample)
	}
}

func observeRankSamples(state *RankState, samples []float64, out []float64) {
	for index, sample := range samples {
		out[index] = ObserveRank(state, sample)
	}
}
