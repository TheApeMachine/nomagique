package core

/*
StageParser splits pipeline stage inputs into prior output and work samples.
*/
type StageParser struct {
	workScratch []Float64
}

/*
NewStageParser returns a parser with reusable work storage.
*/
func NewStageParser() *StageParser {
	return &StageParser{
		workScratch: make([]Float64, 0, 8),
	}
}

/*
Parse extracts the stage out value and work vector from number inputs.
*/
func (stageParser *StageParser) Parse(
	inputs []Number,
) (Float64, []Float64, error) {
	if len(inputs) == 0 {
		return 0, nil, ErrEmptyInputs
	}

	first, ok := inputs[0].(Float64)

	if !ok {
		return 0, nil, ErrEmptyInputs
	}

	if len(inputs) == 1 {
		return first, nil, nil
	}

	stageParser.workScratch = stageParser.workScratch[:0]

	for _, input := range inputs[1:] {
		sample, sampleOK := input.(Float64)

		if !sampleOK {
			return 0, nil, ErrEmptyInputs
		}

		stageParser.workScratch = append(stageParser.workScratch, sample)
	}

	return first, stageParser.workScratch, nil
}
