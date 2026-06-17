package probability

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/dmt"
)

/*
DMTSurprise scores category surprisal from a radix-trie memory of classifier transitions.
*/
type DMTSurprise struct {
	artifact     *datura.Artifact
	tree         *dmt.Tree
	scratch      *dmt.ClassificationScratch
	numStates    int
	lastCategory int
}

/*
NewDMTSurprise returns a DMT-backed surprisal stage for io.ReadWriter pipelines.
*/
func NewDMTSurprise(tree *dmt.Tree, numStates int) *DMTSurprise {
	if numStates < 2 {
		numStates = 4
	}

	if tree == nil {
		var treeErr error

		tree, treeErr = dmt.NewTree("")

		if treeErr != nil {
			tree, _ = dmt.NewTree("")
		}
	}

	return &DMTSurprise{
		artifact:  datura.Acquire("dmt-transition", datura.Artifact_Type_json),
		tree:      tree,
		scratch:   &dmt.ClassificationScratch{},
		numStates: numStates,
	}
}

func (stage *DMTSurprise) Write(payload []byte) (int, error) {
	return stage.artifact.Write(payload)
}

func (stage *DMTSurprise) Read(payload []byte) (int, error) {
	rehydrateArtifact(&stage.artifact, "dmt-transition", datura.Artifact_Type_json)

	if stage == nil || stage.tree == nil || stage.scratch == nil {
		return stage.artifact.Read(payload)
	}

	categoryIndex, categoryOK := datura.PeekOK[int](stage.artifact, "classifier.category")

	if !categoryOK || categoryIndex < 1 || categoryIndex > stage.numStates {
		return stage.artifact.Read(payload)
	}

	sequence := categorySequence(categoryIndex)
	inference := stage.tree.Classify(sequence, stage.scratch)
	surprise := dmtSurpriseFromInference(stage.tree, sequence, inference)

	if stage.lastCategory > 0 && stage.lastCategory != categoryIndex {
		ambiguity := stage.tree.MeasureBranchAmbiguity(sequence)

		if ambiguity.EntropyBits > surprise {
			surprise = ambiguity.EntropyBits
		}

		if surprise <= 0 && stage.numStates > 1 {
			surprise = math.Log2(float64(stage.numStates))
		}
	}

	stage.lastCategory = categoryIndex

	_, _, _ = stage.tree.UnsupervisedLearn(sequence, stage.scratch)

	out := encodePayload(surprise)
	_ = stage.artifact.SetPayload(out)
	pokeFloat(stage.artifact, "transition.surprise", surprise)

	return stage.artifact.Read(payload)
}

func (stage *DMTSurprise) Close() error {
	if stage == nil || stage.tree == nil {
		return nil
	}

	return stage.tree.Close()
}

/*
Reset clears trie sensory and basin statistics for a fresh surprise baseline.
*/
func (stage *DMTSurprise) Reset() error {
	if stage == nil {
		return nil
	}

	if stage.tree != nil {
		return stage.tree.Close()
	}

	return nil
}

func categorySequence(categoryIndex int) []byte {
	return []byte{'c', byte(categoryIndex)}
}

func dmtSurpriseFromInference(
	tree *dmt.Tree,
	sequence []byte,
	inference dmt.ClassificationResult,
) float64 {
	if len(inference.Scores) >= 2 {
		evidence := tree.ComputeBasinContrastiveEvidence(
			inference.Scores[0].ClassName,
			inference.Scores[1].ClassName,
			sequence,
		)

		if evidence.Divergence > 0 && !math.IsNaN(evidence.Divergence) {
			return evidence.Divergence
		}
	}

	if inference.Highest > 0 {
		probability := math.Max(inference.Highest, math.SmallestNonzeroFloat64)

		return -math.Log2(probability)
	}

	ambiguity := tree.MeasureBranchAmbiguity(sequence)

	if ambiguity.Threshold > 0 {
		return ambiguity.EntropyBits / ambiguity.Threshold
	}

	return 0
}
