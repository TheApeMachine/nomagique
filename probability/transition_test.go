package probability

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

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

func TestKLDivergence(testingTB *testing.T) {
	Convey("Given two finite probability distributions", testingTB, func() {
		got, err := klDivergence(
			[]float64{0.5, 0.5},
			[]float64{0.25, 0.75},
		)

		expected := 0.5*math.Log(0.5/0.25) + 0.5*math.Log(0.5/0.75)

		Convey("It should compute the KL divergence directly", func() {
			So(err, ShouldBeNil)
			So(got, ShouldAlmostEqual, expected)
		})
	})

	Convey("Given mismatched probability distributions", testingTB, func() {
		_, err := klDivergence(
			[]float64{0.5, 0.5},
			[]float64{1.0},
		)

		Convey("It should reject the inputs", func() {
			So(err, ShouldNotBeNil)
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
			So(sumProbabilities(padded), ShouldAlmostEqual, 1.0, 1e-9)
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

func TestTransitionSurpriseMeasure(testingTB *testing.T) {
	Convey("Given a typed transition stage", testingTB, func() {
		stage := NewTransitionSurprise(TransitionConfig{
			NumStates: 5,
			Alpha:     0.1,
		})
		output, err := stage.Measure(TransitionInput{
			Probabilities: []float64{0.25, 0.25, 0.25, 0.25},
			Category:      2,
		})

		Convey("It should return finite surprisal and retained counts", func() {
			So(err, ShouldBeNil)
			So(math.IsNaN(output.Value), ShouldBeFalse)
			So(output.Ready, ShouldBeTrue)
			So(stage.matrix.lastCategory, ShouldEqual, 1)
			So(output.Counts[0][1], ShouldBeGreaterThan, 0.1)
		})
	})
}

func TestTransitionSurpriseReset(testingTB *testing.T) {
	Convey("Given a transition stage with accumulated state", testingTB, func() {
		stage := NewTransitionSurprise(TransitionConfig{
			NumStates: 5,
			Alpha:     0.1,
		})
		_, err := stage.Measure(TransitionInput{
			Probabilities: []float64{0.25, 0.25, 0.25, 0.25},
			Category:      2,
		})

		So(err, ShouldBeNil)

		err = stage.Reset()

		Convey("It should clear retained transition state", func() {
			So(err, ShouldBeNil)
			So(stage.matrix.lastCategory, ShouldEqual, 0)
			So(stage.matrix.counts[0][1], ShouldEqual, 0.1)
		})
	})
}

func BenchmarkTransitionSurpriseMeasure(testingTB *testing.B) {
	stage := NewTransitionSurprise(TransitionConfig{
		NumStates: 5,
		Alpha:     0.1,
	})
	input := TransitionInput{
		Probabilities: []float64{0.4, 0.3, 0.2, 0.1},
		Category:      2,
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = stage.Measure(input)
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

func sumProbabilities(probabilities []float64) float64 {
	sum := 0.0

	for _, probability := range probabilities {
		sum += probability
	}

	return sum
}
