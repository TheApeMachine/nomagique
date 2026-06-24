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
			artifact := scalarWire(datura.Acquire("test", datura.APPJSON), "sample", 1)
			posterior := nomagique.Number(probability.NewBernoulli(bernoulliConfig("bernoulli-config")))

			err := transport.NewFlipFlop(artifact, posterior)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0.5)
		})

		Convey("When Rank streams two samples", func() {
			empirical := nomagique.Number(probability.NewRank(rankConfig("rank-config")))
			var artifact *datura.Artifact

			for _, sample := range []float64{10, 5} {
				artifact = scalarWire(datura.Acquire("test", datura.APPJSON), "sample", sample)
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
					Poke("output", "scoreRoot").
					Poke(
						[]string{"s0", "s1", "s2"},
						"inputs",
					),
			)
			transition := probability.NewTransitionSurprise(
				datura.Acquire("transition", datura.APPJSON).
					Poke(float64(3), "numStates").
					Poke(0.1, "alpha"),
			)
			pipeline := nomagique.Number(
				classifier,
				transition,
			)
			artifact := artifactWithScores(map[string]float64{
				"s0":       0.1,
				"s1":       0.8,
				"s2":       0.2,
				"strength": 0.8,
			})

			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)

			surprise := datura.Peek[float64](artifact, "output", "value")

			So(surprise, ShouldBeGreaterThan, 0)
			So(math.IsNaN(surprise), ShouldBeFalse)
			So(int(datura.Peek[float64](artifact, "output", "category")), ShouldEqual, 2)
		})
	})
}

func scalarWire(artifact *datura.Artifact, input string, sample float64) *datura.Artifact {
	artifact.Poke("features", "root")
	artifact.Poke([]string{input}, "inputs")
	artifact.Merge("features", []float64{sample})

	return artifact
}

func bernoulliConfig(name string) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).Poke("sample", "sampleKey")
}

func rankConfig(name string) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).Poke("sample", "sampleKey")
}
