package algorithm_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/algorithm"
)

func pearlConfig() algorithm.PearlConfig {
	return algorithm.PearlConfig{
		Target:          2,
		Treatment:       1,
		Controls:        []int{0},
		MinHistory:      6,
		History:         16,
		CategoryIndexes: []float64{1, 2, 3, 4},
	}
}

func TestPearl_Measure(t *testing.T) {
	Convey("Given numeric rows with a treatment effect", t, func() {
		pearl := algorithm.NewPearl(pearlConfig())
		var output algorithm.PearlOutput
		var ready bool
		var err error

		for index := range 12 {
			control := float64(index % 3)
			treatment := float64(index)
			target := 0.5*control + 2*treatment

			output, ready, err = pearl.Measure(algorithm.PearlInput{
				Key:          "primary",
				Row:          []float64{control, treatment, target},
				Intervention: 20,
			})
		}

		Convey("It emits Pearl ladder and do-calculus evidence", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeTrue)
			So(output.AssociationScore, ShouldBeGreaterThan, 0)
			So(output.InterventionScore, ShouldBeGreaterThan, 0)
			So(output.DoExpectation, ShouldBeGreaterThan, 0)
			So(output.Counterfactual, ShouldBeGreaterThan, output.DoExpectation/2)
			So(output.UpliftScore, ShouldBeGreaterThan, 0)
			So(output.Probabilities, ShouldHaveLength, 4)
		})
	})
}

func TestPearlSample_Measure(t *testing.T) {
	Convey("Given rows for two keys", t, func() {
		sample := algorithm.NewPearlSample(pearlConfig())
		var primary algorithm.PearlSampleOutput

		for index := range 6 {
			var ready bool
			var err error
			primary, ready, err = sample.Measure(algorithm.PearlInput{
				Key: "primary",
				Row: []float64{float64(index % 3), float64(index), float64(index * 2)},
			})

			So(err, ShouldBeNil)
			So(ready, ShouldEqual, index >= 5)
		}

		secondary, ready, err := sample.Measure(algorithm.PearlInput{
			Key: "secondary",
			Row: []float64{1, 2, 3},
		})

		Convey("It keeps rolling rows separate by key", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeFalse)
			So(primary.Key, ShouldEqual, "primary")
			So(secondary.Key, ShouldEqual, "secondary")
			So(primary.Rows, ShouldHaveLength, 6)
			So(secondary.Rows, ShouldHaveLength, 1)
		})
	})
}

func BenchmarkPearl_Measure(t *testing.B) {
	pearl := algorithm.NewPearl(pearlConfig())

	t.ReportAllocs()

	for t.Loop() {
		for index := range 12 {
			control := float64(index % 3)
			treatment := float64(index)
			target := 0.5*control + 2*treatment
			_, _, _ = pearl.Measure(algorithm.PearlInput{
				Key:          "primary",
				Row:          []float64{control, treatment, target},
				Intervention: 20,
			})
		}
	}
}
