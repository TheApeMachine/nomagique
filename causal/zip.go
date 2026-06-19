package causal

/*
ZipNodeRows transposes aligned per-node streams into observation rows.
*/
func ZipNodeRows(streams [][]float64) ([][]float64, bool) {
	if len(streams) == 0 {
		return nil, false
	}

	length := len(streams[0])

	if length == 0 {
		return nil, false
	}

	for _, stream := range streams {
		if len(stream) != length {
			return nil, false
		}
	}

	rows := make([][]float64, length)

	for index := range rows {
		rows[index] = make([]float64, len(streams))

		for nodeIndex, stream := range streams {
			rows[index][nodeIndex] = stream[index]
		}
	}

	return rows, true
}

func alignedStreamLength(streams [][]float64) int {
	length := 0

	for nodeIndex := range streams {
		streamLength := len(streams[nodeIndex])

		if streamLength == 0 {
			return 0
		}

		if length == 0 || streamLength < length {
			length = streamLength
		}
	}

	return length
}
