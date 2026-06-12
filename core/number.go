package core

/*
Float64 is the boundary scalar and a nested pipeline token.
*/
type Float64 float64

/*
Observe runs a nested pipeline registered for this boundary token.
*/
func (boundary Float64) Observe(inputs ...Number) Float64 {
	sample, err := boundary.sampleFromInputs(inputs)

	if err != nil {
		return 0
	}

	stacks, registered := DefaultRegistry.stacks[boundary]

	if !registered || len(stacks) == 0 {
		return sample
	}

	result, err := stacks[len(stacks)-1].Observe(sample)

	if err != nil {
		return 0
	}

	return result
}

/*
Reset is a no-op at the boundary.
*/
func (boundary Float64) Reset() error {
	return nil
}

func (boundary Float64) sampleFromInputs(inputs []Number) (Float64, error) {
	if len(inputs) == 0 {
		return 0, ErrEmptyInputs
	}

	sample, ok := inputs[0].(Float64)

	if !ok {
		return 0, ErrEmptyInputs
	}

	return sample, nil
}
