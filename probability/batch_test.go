package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveBetaSamples(testingTB *testing.T) {
	Convey("Given a beta state", testingTB, func() {
		state := BetaState{}
		outcomes := []float64{1, 0, 1}
		out := make([]float64, len(outcomes))

		observeBetaSamples(&state, outcomes, out)

		Convey("It should match sequential ObserveBeta", func() {
			expect := BetaState{}

			for index, outcome := range outcomes {
				expectValue := ObserveBeta(&expect, outcome)
				So(out[index], ShouldEqual, expectValue)
			}
		})
	})
}

func TestObserveBetaPairSamples(testingTB *testing.T) {
	Convey("Given a beta state and paired outcomes", testingTB, func() {
		state := BetaState{}
		predicted := []float64{10, 10}
		actual := []float64{12, 8}
		out := make([]float64, len(predicted))

		observeBetaPairSamples(&state, predicted, actual, out)

		Convey("It should match sequential ObserveBetaPair", func() {
			expect := BetaState{}

			for index := range predicted {
				expectValue := ObserveBetaPair(&expect, predicted[index], actual[index])
				So(out[index], ShouldEqual, expectValue)
			}
		})
	})
}

func TestObserveCUSUMSamples(testingTB *testing.T) {
	Convey("Given a CUSUM state", testingTB, func() {
		state := CUSUMState{}
		samples := []float64{0.1, -0.2, 0.3}
		out := make([]float64, len(samples))

		observeCUSUMSamples(&state, samples, out)

		Convey("It should match sequential ObserveCUSUM", func() {
			expect := CUSUMState{}

			for index, sample := range samples {
				expectValue := ObserveCUSUM(&expect, sample)
				So(out[index], ShouldEqual, expectValue)
			}
		})
	})
}

func TestObserveRankSamples(testingTB *testing.T) {
	Convey("Given a rank state", testingTB, func() {
		state := RankState{}
		samples := []float64{3, 1, 2}
		out := make([]float64, len(samples))

		observeRankSamples(&state, samples, out)

		Convey("It should match sequential ObserveRank", func() {
			expect := RankState{}

			for index, sample := range samples {
				expectValue := ObserveRank(&expect, sample)
				So(out[index], ShouldEqual, expectValue)
			}
		})
	})
}
