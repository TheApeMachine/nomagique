package probability_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/probability"
)

func TestProbabilityTypedIntegration(testingTB *testing.T) {
	Convey("Given typed probability stages", testingTB, func() {
		posterior := probability.NewBernoulli()
		rank := probability.NewRank()
		softmax := probability.NewSoftmax(probability.SoftmaxConfig{
			Inputs: []string{"trend", "reversal", "noise"},
		})
		transition := probability.NewTransitionSurprise(probability.TransitionConfig{
			NumStates: 4,
			Alpha:     0.1,
		})

		bayes, bayesErr := posterior.Measure(1)
		empirical, rankErr := rank.Measure(0.8)
		probabilities, softmaxErr := softmax.Measure(probability.SoftmaxInput{
			Scores: []probability.CategoryScore{
				{Category: "trend", Score: bayes.Value},
				{Category: "reversal", Score: empirical.Value},
				{Category: "noise", Score: 0.1},
			},
		})
		surprise, transitionErr := transition.Measure(probability.TransitionInput{
			Probabilities: probabilities.Values,
			Category:      probability.ArgmaxIndex(probabilities.Values) + 1,
		})

		Convey("It should compose through typed probability outputs", func() {
			So(bayesErr, ShouldBeNil)
			So(rankErr, ShouldBeNil)
			So(softmaxErr, ShouldBeNil)
			So(transitionErr, ShouldBeNil)
			So(bayes.Value, ShouldBeGreaterThan, 0.5)
			So(empirical.Value, ShouldEqual, 1)
			So(len(probabilities.Values), ShouldEqual, 3)
			So(surprise.Ready, ShouldBeTrue)
		})
	})
}
