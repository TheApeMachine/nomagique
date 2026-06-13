package learning

func observeSampleRatioSamples(
	state *SampleRatioState,
	predicted []float64,
	actual []float64,
	out []float64,
) {
	for index, predict := range predicted {
		out[index] = ObserveSampleRatio(state, predict, actual[index])
	}
}

func observeWeightSamples(
	state *WeightState,
	predicted []float64,
	actual []float64,
	out []float64,
) {
	for index, predict := range predicted {
		out[index] = ObserveWeight(state, predict, actual[index])
	}
}

func observeForecastSamples(
	state *ForecastState,
	predicted []float64,
	actual []float64,
	out []float64,
) {
	for index, predict := range predicted {
		out[index] = ObserveForecast(state, predict, actual[index])
	}
}
