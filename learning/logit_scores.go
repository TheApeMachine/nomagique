package learning

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
LogitScores maps configured feature outputs to classifier logits on the artifact.
Runtime scale and threshold history live on the config artifact output keys.
*/
type LogitScores struct {
	config *datura.Artifact
}

/*
NewLogitScores returns a classifier logit stage configured on the artifact.
*/
func NewLogitScores(config *datura.Artifact) *LogitScores {
	return &LogitScores{
		config: config,
	}
}

func (logitScores *LogitScores) Read(payload []byte) (int, error) {
	state := datura.Acquire("logit-scores-state", datura.APPJSON)

	if _, err := state.Write(logitScores.config.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: state write failed",
			err,
		))
	}

	defer state.Release()

	order := datura.Peek[[]string](logitScores.config, "order")
	outputs := datura.Peek[[]string](logitScores.config, "outputs")

	if len(order) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: config order required",
			nil,
		))
	}

	if len(outputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"logit-scores: config outputs required",
			nil,
		))
	}

	features, err := logitScores.featureValues(state, order)

	if err != nil {
		return 0, err
	}

	threshold, err := logitScores.resolveThreshold(features, state)

	if err != nil {
		return 0, err
	}

	scales, err := logitScores.featureScales(order, features, state)

	if err != nil {
		return 0, err
	}

	weights, err := NewClassifierWeights(logitScores.config, threshold, scales)

	if err != nil {
		return 0, err
	}

	centeredFeatures := logitScores.centeredFeatures(features, scales)
	scores := weights.Scores(centeredFeatures)

	if err := logitScores.applyDecline(
		state, logitScores.config, weights, centeredFeatures, scores,
	); err != nil {
		return 0, err
	}

	for index, outputKey := range outputs {
		score, err := logitScores.resolveOutputScore(
			state, outputKey, scores[index], centeredFeatures, scales,
		)

		if err != nil {
			return 0, err
		}

		state.MergeOutput(outputKey, score)
	}

	state.MergeOutput("value", scores[0])

	strength := scores[0]

	for _, score := range scores[1:] {
		if score > strength {
			strength = score
		}
	}

	state.MergeOutput("strength", strength)
	state.Poke("output", "root")

	outputInputs := make([]string, 0, len(outputs)+1)
	outputInputs = append(outputInputs, outputs...)
	outputInputs = append(outputInputs, "strength")
	state.Poke(outputInputs, "inputs")

	return state.Read(payload)
}

func (logitScores *LogitScores) Write(payload []byte) (int, error) {
	logitScores.config.WithPayload(payload)
	return len(payload), nil
}

func (logitScores *LogitScores) Close() error {
	return nil
}
