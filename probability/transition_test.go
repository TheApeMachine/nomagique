package probability

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

func TestTransitionMatrixSurprise(testingTB *testing.T) {
	Convey("Given a transition matrix and padded observation", testingTB, func() {
		matrix := NewTransitionMatrix(5, 0.1)
		observed := matrix.PadObserved([]float64{0.25, 0.25, 0.25, 0.25}, 0)

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
		padded := matrix.PadObserved([]float64{0.4, 0.3, 0.2, 0.1}, 0)

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

func TestTransitionSurprise_Read(testingTB *testing.T) {
	Convey("Given a padded observation through TransitionSurprise", testingTB, func() {
		stage := NewTransitionSurprise(5, 0.1)
		matrix := stage.matrix
		observed := matrix.PadObserved([]float64{0.25, 0.25, 0.25, 0.25}, 0)
		inbound := datura.Acquire("transition-test", datura.Artifact_Type_json)
		pokeFloatList(inbound, "classifier.probabilities", observed)
		pokeInt(inbound, "classifier.category", 1)
		buf, _ := inbound.Message().Marshal()
		_, _ = stage.Write(buf)

		got := readScalar(stage)

		Convey("It should return finite surprisal", func() {
			So(math.IsNaN(got), ShouldBeFalse)
		})
	})
}

func TestTransitionSurprise_Reset(testingTB *testing.T) {
	Convey("Given a reset transition stage", testingTB, func() {
		stage := NewTransitionSurprise(5, 0.1)
		stage.matrix.Update(2)

		err := stage.Reset()

		Convey("It should clear matrix state", func() {
			So(err, ShouldBeNil)
			So(stage.matrix.lastCategory, ShouldEqual, 0)
		})
	})
}

func BenchmarkTransitionSurprise_Read(testingTB *testing.B) {
	stage := NewTransitionSurprise(5, 0.1)
	matrix := stage.matrix
	observed := matrix.PadObserved([]float64{0.4, 0.3, 0.2, 0.1}, 0)
	inbound := datura.Acquire("transition-bench", datura.Artifact_Type_json)
	pokeFloatList(inbound, "classifier.probabilities", observed)
	pokeInt(inbound, "classifier.category", 2)
	buf, _ := inbound.Message().Marshal()

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = stage.Write(buf)
		_ = readScalar(stage)
	}
}

func BenchmarkTransitionMatrixSurprise(testingTB *testing.B) {
	matrix := NewTransitionMatrix(5, 0.1)
	observed := matrix.PadObserved([]float64{0.4, 0.3, 0.2, 0.1}, 0)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = matrix.Surprise(observed)
	}
}
