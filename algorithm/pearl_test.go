package algorithm_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/algorithm"
)

func pearlConfig() algorithm.PearlConfig {
	return algorithm.PearlConfig{
		Target:    2,
		Treatment: 1,
		Controls:  []int{0},
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
			So(err, ShouldBeNil)

			if index < 2 {
				So(ready, ShouldBeFalse)
				So(output, ShouldResemble, algorithm.PearlOutput{})
			}
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

	Convey("Given equivalent return systems expressed on different numeric scales", t, func() {
		base := algorithm.NewPearl(algorithm.PearlConfig{Target: 1, Treatment: 0})
		rescaled := algorithm.NewPearl(algorithm.PearlConfig{Target: 1, Treatment: 0})
		var baseOutput algorithm.PearlOutput
		var rescaledOutput algorithm.PearlOutput
		var baseReady bool
		var rescaledReady bool
		var baseErr error
		var rescaledErr error

		for index := range 24 {
			treatment := float64(index+1) * 0.0001
			target := 0.75*treatment + float64(index%3-1)*0.00001
			baseOutput, baseReady, baseErr = base.Measure(algorithm.PearlInput{
				Key: "base", Row: []float64{treatment, target},
			})
			rescaledOutput, rescaledReady, rescaledErr = rescaled.Measure(algorithm.PearlInput{
				Key: "rescaled", Row: []float64{treatment * 0.000001, target * 1000000},
			})
		}

		Convey("It should preserve evidence strength, shares, and gates", func() {
			So(baseErr, ShouldBeNil)
			So(rescaledErr, ShouldBeNil)
			So(baseReady, ShouldBeTrue)
			So(rescaledReady, ShouldBeTrue)
			So(rescaledOutput.Strength, ShouldAlmostEqual, baseOutput.Strength, 1e-10)
			So(rescaledOutput.Confidence, ShouldAlmostEqual, baseOutput.Confidence, 1e-10)
			So(rescaledOutput.EntryBaseline, ShouldAlmostEqual,
				baseOutput.EntryBaseline, 1e-10)
			So(rescaledOutput.Residual(), ShouldAlmostEqual, baseOutput.Residual(), 1e-10)
		})
	})

	Convey("Given a treatment perfectly explained by a control", t, func() {
		pearl := algorithm.NewPearl(algorithm.PearlConfig{
			Target: 2, Treatment: 1, Controls: []int{0},
		})
		ready := false
		var output algorithm.PearlOutput

		for index := range 24 {
			control := float64(index) * 0.0001
			var err error
			output, ready, err = pearl.Measure(algorithm.PearlInput{
				Key: "collinear",
				Row: []float64{control, control, 0.5 * control},
			})

			So(err, ShouldBeNil)
		}

		Convey("It should remain unresolved instead of publishing unstable evidence", func() {
			So(ready, ShouldBeFalse)
			So(output, ShouldResemble, algorithm.PearlOutput{})
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
			So(ready, ShouldEqual, index >= 1)
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

	Convey("Given a sustained causal stream", t, func() {
		sample := algorithm.NewPearlSample(pearlConfig())
		observations := 4096
		var latest algorithm.PearlSampleOutput

		for index := range observations {
			var err error
			latest, _, err = sample.Measure(algorithm.PearlInput{
				Key: "primary",
				Row: []float64{float64(index % 3), float64(index), float64(index * 2)},
			})
			So(err, ShouldBeNil)
		}

		Convey("It retains a bounded adaptive table ending at the latest observation", func() {
			So(len(latest.Rows), ShouldBeLessThan, observations)
			So(latest.Rows[len(latest.Rows)-1], ShouldResemble, latest.Row)
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
