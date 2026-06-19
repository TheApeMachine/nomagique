package probability_test

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/probability"
)

func TestIntegration(t *testing.T) {
	Convey("Given probability stages composed through nomagique.Number", t, func() {
		Convey("When Bernoulli observes a unit success", func() {
			artifact := datura.Acquire("test", datura.APPJSON)
			posterior := nomagique.Number(probability.NewBernoulli())

			artifact.Poke(1, "sample")
			err := transport.NewFlipFlop(artifact, posterior)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0.5)
		})

		Convey("When Rank streams two samples", func() {
			artifact := datura.Acquire("test", datura.APPJSON)
			empirical := nomagique.Number(probability.NewRank())

			for _, sample := range []float64{10, 5} {
				artifact.Poke(sample, "sample")
				err := transport.NewFlipFlop(artifact, empirical)

				So(err, ShouldBeNil)
			}

			got := datura.Peek[float64](artifact, "output", "value")

			So(got, ShouldBeGreaterThan, 0)
			So(got, ShouldBeLessThan, 1)
		})

		Convey("When Classifier and Transition run in sequence", func() {
			classifier := probability.NewClassifier(
				datura.Acquire("schema", datura.APPJSON).
					RetainStageAttributes().
					Poke(
						[]string{"s0", "s1", "s2"},
						"inputs",
					),
			)
			pipeline := nomagique.Number(
				classifier,
				probability.NewTransitionSurprise(3, 0.1),
			)
			artifact := datura.Acquire("test", datura.APPJSON).
				Poke(0.1, "output", "s0").
				Poke(0.8, "output", "s1").
				Poke(0.2, "output", "s2")

			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)

			surprise := datura.Peek[float64](artifact, "transition", "surprise")

			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, surprise)
			So(math.IsNaN(surprise), ShouldBeFalse)
			So(datura.Peek[int](artifact, "classifier", "category"), ShouldEqual, 2)
		})
	})
}
