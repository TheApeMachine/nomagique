package probability

import "github.com/theapemachine/errnie"

/*
TransitionConfig describes a typed transition model.
*/
type TransitionConfig struct {
	NumStates int
	Alpha     float64
}

/*
TransitionInput carries one classifier distribution and selected category.
*/
type TransitionInput struct {
	Probabilities []float64
	Category      int
}

/*
TransitionOutput reports transition surprisal and retained matrix state.
*/
type TransitionOutput struct {
	Value         float64
	Category      int
	Probabilities []float64
	Counts        [][]float64
	Ready         bool
}

/*
Transition scores transition surprisal from classifier probabilities.
*/
type Transition struct {
	config TransitionConfig
	matrix *TransitionMatrix
}

/*
NewTransitionSurprise returns a typed transition surprisal stage.
*/
func NewTransitionSurprise(configs ...TransitionConfig) *Transition {
	config := TransitionConfig{}

	if len(configs) > 0 {
		config = configs[0]
	}

	return &Transition{
		config: config,
	}
}

/*
Measure scores transition surprise and records the selected category.
*/
func (transition *Transition) Measure(input TransitionInput) (TransitionOutput, error) {
	if err := transition.ready(); err != nil {
		return TransitionOutput{}, err
	}

	if len(input.Probabilities) == 0 {
		return TransitionOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"transition: probabilities required",
			nil,
		))
	}

	for _, probability := range input.Probabilities {
		if err := finiteProbability("transition", probability); err != nil {
			return TransitionOutput{}, err
		}
	}

	observed, err := transition.matrix.PadObserved(
		input.Probabilities,
		transition.config.Alpha,
	)

	if err != nil {
		return TransitionOutput{}, err
	}

	surprise, err := transition.matrix.Surprise(observed)

	if err != nil {
		return TransitionOutput{}, err
	}

	if input.Category >= 1 && input.Category <= transition.config.NumStates {
		transition.matrix.Update(input.Category - 1)
	}

	return TransitionOutput{
		Value:         surprise,
		Category:      input.Category,
		Probabilities: append([]float64(nil), input.Probabilities...),
		Counts:        transition.Counts(),
		Ready:         true,
	}, nil
}

/*
Reset clears retained transition counts.
*/
func (transition *Transition) Reset() error {
	if err := transition.ready(); err != nil {
		return err
	}

	transition.matrix.Reset()

	return nil
}

/*
Counts returns a copy of the retained transition matrix counts.
*/
func (transition *Transition) Counts() [][]float64 {
	if transition == nil || transition.matrix == nil {
		return nil
	}

	counts := make([][]float64, len(transition.matrix.counts))

	for row := range transition.matrix.counts {
		counts[row] = append([]float64(nil), transition.matrix.counts[row]...)
	}

	return counts
}

func (transition *Transition) ready() error {
	if transition == nil {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"transition: config required",
			nil,
		))
	}

	if transition.config.NumStates <= 0 || transition.config.Alpha <= 0 {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			"transition: numStates and alpha must be positive",
			nil,
		))
	}

	if err := finiteProbability("transition", transition.config.Alpha); err != nil {
		return err
	}

	if transition.matrix == nil {
		transition.matrix = NewTransitionMatrix(
			transition.config.NumStates,
			transition.config.Alpha,
		)
	}

	return nil
}
