package core

/*
Number is a reactive numeric primitive. Observe applies pipeline stages to the
carried sample; Reset clears derived state.
*/
type Number interface {
	Observe(...Number) Float64
	Reset() error
}

/*
Numbers is a slice of numbers.
*/
type Numbers []Number

/*
Float64 converts the numbers to a slice of float64s.
*/
func (numbers Numbers) Float64() []float64 {
	floats := make([]float64, len(numbers))

	for index, number := range numbers {
		sample, isBoundary := number.(Float64)

		if isBoundary {
			floats[index] = float64(sample)
			continue
		}

		floats[index] = float64(number.Observe())
	}

	return floats
}
