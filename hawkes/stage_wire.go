package hawkes

/*
Feeds algorithm/excitation and equation Hawkes stages — EncodeMomentBatch turns marked trade arrivals into fitted intensities and categories.
*/
func EncodeMomentBatch(xStream, yStream []float64) []float64 {
	if len(xStream) != len(yStream) || len(xStream) < 2 {
		return nil
	}

	batch := make([]float64, 0, len(xStream)+len(yStream))
	batch = append(batch, xStream...)
	batch = append(batch, yStream...)

	return batch
}

/*
Feeds algorithm/excitation and equation Hawkes stages — EncodeFitBatch fits marked trade arrivals into intensities and fit categories.
*/

func EncodeFitBatch(xTimes, yTimes []float64) []float64 {
	if len(xTimes)+len(yTimes) < 2 {
		return nil
	}

	batch := make([]float64, 0, len(xTimes)+len(yTimes))
	batch = append(batch, xTimes...)
	batch = append(batch, yTimes...)

	return batch
}
