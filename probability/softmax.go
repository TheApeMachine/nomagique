package probability

import (
	"fmt"
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Softmax maps competing scores on the wire to a normalized probability vector.
Every configured input key receives its probability on output; nothing is
pre-selected as a winner.
*/
type Softmax struct {
	artifact *datura.Artifact
}

/*
NewSoftmax returns a softmax stage wired from schema inputs on the artifact.
Set normalize to zero on the config artifact to use raw logits; default is
spread-normalized scores.
*/
func NewSoftmax(artifact *datura.Artifact) *Softmax {
	return &Softmax{artifact: artifact}
}

func (softmax *Softmax) Read(payload []byte) (int, error) {
	state := datura.Acquire("softmax-state", datura.APPJSON)

	if _, err := state.Unpack(softmax.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"softmax: state write failed",
			err,
		))
	}

	defer state.Release()

	inputs := datura.Peek[[]string](softmax.artifact, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"softmax: inputs required",
			nil,
		))
	}

	scoreRoot := datura.Peek[string](softmax.artifact, "scoreRoot")

	if scoreRoot == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"softmax: scoreRoot required",
			nil,
		))
	}

	scores := make([]float64, len(inputs))

	for index, input := range inputs {
		if input == "" {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"softmax: empty input key",
				nil,
			))
		}

		var score float64
		scoreFound := false

		for wireIndex, wireInput := range datura.Peek[[]string](state, "inputs") {
			if wireInput != input {
				continue
			}

			if scoreRoot == "features" {
				features := datura.Peek[[]float64](state, scoreRoot)

				if wireIndex >= len(features) {
					return 0, errnie.Error(errnie.Err(
						errnie.Validation,
						"softmax: feature index out of range",
						nil,
					))
				}

				score = features[wireIndex]
			}

			if scoreRoot != "features" {
				score = datura.Peek[float64](state, scoreRoot, wireInput)
			}

			scoreFound = true
		}

		if !scoreFound {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"softmax: input not in inputs",
				nil,
			))
		}

		if math.IsNaN(score) || math.IsInf(score, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"softmax: score is non-finite",
				nil,
			))
		}

		scores[index] = score
	}

	normalize := datura.Peek[float64](softmax.artifact, "normalize") != 0

	if datura.Peek[string](softmax.artifact, "normalize") == "false" {
		normalize = false
	}

	probabilities, err := softmax.distribute(scores, normalize)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"softmax: unable to compute probabilities",
			err,
		))
	}

	for index, input := range inputs {
		state.MergeOutput(input, probabilities[index])
	}

	state.MergeOutput("probabilities", probabilities)
	state.Poke("output", "root")

	outputInputs := make([]string, 0, len(inputs)+1)
	outputInputs = append(outputInputs, inputs...)
	outputInputs = append(outputInputs, "probabilities")
	state.Poke(outputInputs, "inputs")

	return state.PackInto(payload)
}

func (softmax *Softmax) Write(payload []byte) (int, error) {
	softmax.artifact.WithPayload(payload)
	return len(payload), nil
}

func (softmax *Softmax) Close() error {
	return nil
}

func (softmax *Softmax) distribute(scores []float64, normalize bool) ([]float64, error) {
	if len(scores) == 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"softmax: requires at least one score",
			nil,
		))
	}

	for index, score := range scores {
		if math.IsNaN(score) || math.IsInf(score, 0) {
			return nil, errnie.Error(errnie.Err(
				errnie.Validation,
				fmt.Sprintf("softmax: score[%d] is non-finite", index),
				nil,
			))
		}
	}

	if normalize {
		return softmax.normalized(scores)
	}

	return softmax.raw(scores)
}

func (softmax *Softmax) raw(scores []float64) ([]float64, error) {
	probabilities := make([]float64, len(scores))
	maxScore := scores[0]

	for _, score := range scores[1:] {
		maxScore = math.Max(maxScore, score)
	}

	expSum := 0.0

	for index, score := range scores {
		weighted := math.Exp(score - maxScore)
		probabilities[index] = weighted
		expSum += weighted
	}

	if expSum <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"softmax: normalization sum is non-positive",
			nil,
		))
	}

	for index := range probabilities {
		probabilities[index] /= expSum
	}

	return probabilities, nil
}

func (softmax *Softmax) normalized(scores []float64) ([]float64, error) {
	return SoftmaxScoresNormalized(scores)
}

var _ io.ReadWriteCloser = (*Softmax)(nil)
