package probability

import (
	"bytes"
	"io"

	"github.com/theapemachine/datura"
)

/*
Classifier selects a category from competing score stages.

Wire one io.ReadWriter score source per category in NewClassifier. On Read the
stage evaluates every source against the carried artifact, normalizes the scores
with SoftmaxScoresNormalized, and emits a 1-based winning category index.
*/
type Classifier struct {
	artifact      *datura.Artifact
	scoreSources  []io.ReadWriter
	probabilities []float64
	categoryIndex int
}

/*
NewClassifier returns a classifier over scoreSources.

Each source is re-read on every Classifier.Read call so upstream stages such as
algorithm.Pearl can populate ladder readings before classification.
*/
func NewClassifier(scoreSources ...io.ReadWriter) *Classifier {
	if len(scoreSources) == 0 {
		return nil
	}

	return &Classifier{
		artifact:     datura.Acquire("classifier", datura.Artifact_Type_json),
		scoreSources: scoreSources,
	}
}

func (classifier *Classifier) Write(p []byte) (int, error) {
	return classifier.artifact.Write(p)
}

func (classifier *Classifier) Read(p []byte) (int, error) {
	rehydrateArtifact(&classifier.artifact, "classifier", datura.Artifact_Type_json)

	scores, ok := classifier.scores()

	if ok {
		probabilities, err := SoftmaxScoresNormalized(scores)

		if err == nil {
			classifier.probabilities = probabilities
			classifier.categoryIndex = ArgmaxIndex(probabilities) + 1
			out := encodePayload(float64(classifier.categoryIndex))
			_ = classifier.artifact.SetPayload(out)

			pokeFloatList(classifier.artifact, "classifier.probabilities", classifier.probabilities)
			pokeInt(classifier.artifact, "classifier.category", classifier.categoryIndex)

			confidence, confidenceErr := classifier.Confidence(classifier.categoryIndex)

			if confidenceErr == nil {
				pokeFloat(classifier.artifact, "classifier.confidence", confidence)
			}
		}
	}

	return classifier.artifact.Read(p)
}

func (classifier *Classifier) Value() float64 {
	payload, _ := classifier.artifact.Payload()
	value, ok := payloadScalar(payload)

	if !ok {
		return 0
	}

	return value
}

func (classifier *Classifier) Close() error {
	return nil
}

/*
Probabilities returns the normalized category probabilities from the last Read.
*/
func (classifier *Classifier) Probabilities() []float64 {
	return classifier.probabilities
}

/*
CategoryIndex returns the 1-based winning category from the last Read.
*/
func (classifier *Classifier) CategoryIndex() int {
	return classifier.categoryIndex
}

/*
Confidence returns the normalized probability share for categoryIndex.
When categoryIndex is zero the winning category is used.
*/
func (classifier *Classifier) Confidence(categoryIndex int) (float64, error) {
	return CategoryConfidence(classifier.probabilities, categoryIndex)
}

/*
Reset clears derived state.
*/
func (classifier *Classifier) Reset() error {
	classifier.probabilities = nil
	classifier.categoryIndex = 0

	return nil
}

func (classifier *Classifier) scores() ([]float64, bool) {
	if len(classifier.scoreSources) == 0 {
		return nil, false
	}

	buf, err := classifier.inboundBytes()

	if err != nil {
		return nil, false
	}

	scores := make([]float64, len(classifier.scoreSources))

	for index, scoreSource := range classifier.scoreSources {
		score, scoreOK := readScore(scoreSource, buf)

		if !scoreOK {
			return nil, false
		}

		scores[index] = score
	}

	return scores, true
}

func (classifier *Classifier) inboundBytes() ([]byte, error) {
	payload, err := classifier.artifact.Payload()

	if err != nil {
		return nil, err
	}

	inbound := datura.Acquire("classifier-in", datura.Artifact_Type_json)
	_ = inbound.SetPayload(payload)

	return inbound.Message().Marshal()
}

func readScore(scoreSource io.ReadWriter, artifactBytes []byte) (float64, bool) {
	_, writeErr := scoreSource.Write(artifactBytes)

	if writeErr != nil {
		return 0, false
	}

	var outBuf bytes.Buffer
	chunk := make([]byte, 4096)

	for {
		readCount, readErr := scoreSource.Read(chunk)

		if readCount > 0 {
			outBuf.Write(chunk[:readCount])
		}

		if readErr == io.EOF {
			break
		}

		if readErr != nil {
			return 0, false
		}
	}

	outbound := datura.Acquire("classifier-score", datura.Artifact_Type_json)
	_, _ = outbound.Write(outBuf.Bytes())
	payload, payloadErr := outbound.Payload()

	if payloadErr != nil {
		return 0, false
	}

	return payloadScalar(payload)
}
