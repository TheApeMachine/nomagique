package utils

/*
AppendRingFloat appends then drops the left tail so callers get a bounded
rolling window without maintaining head indices in every consumer.
*/
func AppendRingFloat(values []float64, value float64, capacity int) []float64 {
	values = append(values, value)

	if len(values) <= capacity {
		return values
	}

	return values[len(values)-capacity:]
}
