package probability

import (
	"math"
	"testing"

	"github.com/bytedance/sonic"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func transitionSchema(numStates int, alpha float64) *datura.Artifact {
	return datura.Acquire("transition", datura.APPJSON).
		Poke(float64(numStates), "numStates").
		Poke(alpha, "alpha")
}

func TestTransitionMatrixSurprise(testingTB *testing.T) {
	Convey("Given a transition matrix and padded observation", testingTB, func() {
		matrix := NewTransitionMatrix(5, 0.1)
		observed, err := matrix.PadObserved([]float64{0.25, 0.25, 0.25, 0.25}, 0.1)

		So(err, ShouldBeNil)

		surprise, err := matrix.Surprise(observed)

		Convey("It should not return NaN", func() {
			So(err, ShouldBeNil)
			So(math.IsNaN(surprise), ShouldBeFalse)
		})
	})
}

func TestTransitionMatrixPadObserved(testingTB *testing.T) {
	Convey("Given a four-class distribution", testingTB, func() {
		matrix := NewTransitionMatrix(5, 0.1)
		padded, err := matrix.PadObserved([]float64{0.4, 0.3, 0.2, 0.1}, 0.1)

		So(err, ShouldBeNil)

		Convey("It should produce five normalized masses", func() {
			So(len(padded), ShouldEqual, 5)

			sum := 0.0

			for _, probability := range padded {
				sum += probability
			}

			So(sum, ShouldAlmostEqual, 1.0, 1e-9)
		})
	})
}

func TestTransitionMatrixUpdate(testingTB *testing.T) {
	Convey("Given a transition update", testingTB, func() {
		matrix := NewTransitionMatrix(5, 0.1)

		matrix.Update(2)

		Convey("It should advance last category", func() {
			So(matrix.lastCategory, ShouldEqual, 2)
		})
	})
}

func TestTransitionMatrixReset(testingTB *testing.T) {
	Convey("Given a reset transition matrix", testingTB, func() {
		matrix := NewTransitionMatrix(5, 0.1)
		matrix.Update(2)

		matrix.Reset()

		Convey("It should restore the smoothing prior", func() {
			So(matrix.lastCategory, ShouldEqual, 0)
			So(matrix.counts[0][0], ShouldEqual, 0.1)
		})
	})
}

func transitionInboundArtifact(probabilities []float64, category float64) *datura.Artifact {
	payload, err := sonic.Marshal(datura.Map[any]{
		"output": datura.Map[any]{
			"probabilities": probabilities,
			"category":      category,
		},
	})

	if err != nil {
		panic(err)
	}

	return datura.Acquire("transition-test", datura.APPJSON).WithPayload(payload)
}

func transitionCounts(artifact *datura.Artifact) []float64 {
	rawAttributes, err := artifact.Attributes()

	if err != nil || len(rawAttributes) == 0 {
		return nil
	}

	node, err := sonic.Get(rawAttributes, "transition", "counts")

	if err != nil || !node.Exists() {
		return nil
	}

	rawCounts, err := node.ArrayUseNode()

	if err != nil {
		return nil
	}

	counts := make([]float64, len(rawCounts))

	for index, sample := range rawCounts {
		value, valueErr := sample.Float64()

		if valueErr != nil {
			return nil
		}

		counts[index] = value
	}

	return counts
}

func TestTransitionSurprise_Read(testingTB *testing.T) {
	Convey("Given a padded observation through TransitionSurprise", testingTB, func() {
		stage := NewTransitionSurprise(transitionSchema(5, 0.1))
		matrix := NewTransitionMatrix(5, 0.1)
		observed, err := matrix.PadObserved([]float64{0.25, 0.25, 0.25, 0.25}, 0.1)

		So(err, ShouldBeNil)

		artifact := transitionInboundArtifact(observed, 1)

		err = nomagique.RoundTripArtifact(artifact, stage)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return finite surprisal", func() {
			So(math.IsNaN(got), ShouldBeFalse)
		})
	})
}

func TestTransitionSurprise_Reset(testingTB *testing.T) {
	Convey("Given a transition stage with accumulated state", testingTB, func() {
		stage := NewTransitionSurprise(transitionSchema(5, 0.1))
		matrix := NewTransitionMatrix(5, 0.1)
		observed, err := matrix.PadObserved([]float64{0.25, 0.25, 0.25, 0.25}, 0.1)

		So(err, ShouldBeNil)

		artifact := transitionInboundArtifact(observed, 2)

		err = nomagique.RoundTripArtifact(artifact, stage)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](stage.artifact, "transition", "lastCategory"), ShouldEqual, 1)

		resetPayload, marshalErr := sonic.Marshal(datura.Map[any]{"reset": 1.0})

		So(marshalErr, ShouldBeNil)

		artifact = datura.Acquire("transition-test", datura.APPJSON).WithPayload(resetPayload)
		err = nomagique.RoundTripArtifact(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should clear retained transition state", func() {
			So(datura.Peek[float64](stage.artifact, "transition", "lastCategory"), ShouldEqual, 0)
			So(transitionCounts(stage.artifact), ShouldBeNil)
		})
	})
}

func BenchmarkTransitionSurprise_Read(testingTB *testing.B) {
	stage := NewTransitionSurprise(transitionSchema(5, 0.1))
	matrix := NewTransitionMatrix(5, 0.1)
	observed, err := matrix.PadObserved([]float64{0.4, 0.3, 0.2, 0.1}, 0.1)

	if err != nil {
		testingTB.Fatal(err)
	}

	artifact := transitionInboundArtifact(observed, 2)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = nomagique.RoundTripArtifact(artifact, stage)
	}
}

func BenchmarkTransitionMatrixSurprise(testingTB *testing.B) {
	matrix := NewTransitionMatrix(5, 0.1)
	observed, err := matrix.PadObserved([]float64{0.4, 0.3, 0.2, 0.1}, 0.1)

	if err != nil {
		testingTB.Fatal(err)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = matrix.Surprise(observed)
	}
}
