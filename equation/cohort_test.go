package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/equation"
)

const cohortPayloadHeader = 6

func cohortBatch(
	window int,
	barSpacingSeconds float64,
	energy float64,
	pairCorrelations, peerCorrelations, peerEnergies []float64,
) []float64 {
	batch := make(
		[]float64,
		0,
		cohortPayloadHeader+len(pairCorrelations)+len(peerCorrelations)+len(peerEnergies),
	)
	batch = append(batch, float64(window))
	batch = append(batch,
		float64(len(pairCorrelations)),
		float64(len(peerCorrelations)),
		float64(len(peerEnergies)),
	)
	batch = append(batch, barSpacingSeconds)
	batch = append(batch, energy)
	batch = append(batch, pairCorrelations...)
	batch = append(batch, peerCorrelations...)
	batch = append(batch, peerEnergies...)

	return batch
}

func TestCohort_Measure(testingTB *testing.T) {
	cases := []struct {
		name     string
		batch    []float64
		wantCat  int
		eligible bool
	}{
		{
			name:     "header too short",
			batch:    []float64{3, 2, 2, 2},
			eligible: false,
		},
		{
			name: "noise low energy",
			batch: cohortBatch(
				3,
				60,
				0.1,
				[]float64{0.05, 0.04},
				[]float64{0.5, 0.6, 0.7, 0.8},
				[]float64{0.5, 0.6, 0.7, 0.8},
			),
			wantCat:  3,
			eligible: true,
		},
		{
			name: "herd high correlation and energy",
			batch: cohortBatch(
				4,
				60,
				0.5,
				[]float64{0.8, 0.9},
				[]float64{0.1, 0.2, 0.3, 0.4},
				[]float64{0.1, 0.2, 0.3, 0.4},
			),
			wantCat:  1,
			eligible: true,
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given cohort payload "+testCase.name, testingTB, func() {
			cohort := equation.NewCohort()
			frame := equation.NewFeatureFrame(equation.CohortInputKeys, testCase.batch)
			output, err := cohort.Measure(frame)

			if !testCase.eligible {
				Convey("It should reject invalid payload", func() {
					So(err, ShouldNotBeNil)
				})

				return
			}

			So(err, ShouldBeNil)

			Convey("It should classify the cohort", func() {
				So(output.Strength, ShouldBeGreaterThan, 0)
				So(output.Category, ShouldEqual, testCase.wantCat)

				if testCase.wantCat == 1 {
					So(output.PeakScore, ShouldBeGreaterThan, 0)
				}
			})
		})
	}
}

func BenchmarkCohortMeasure(b *testing.B) {
	cohort := equation.NewCohort()
	values := cohortBatch(
		4,
		60,
		0.5,
		[]float64{0.8, 0.9},
		[]float64{0.1, 0.2, 0.3, 0.4},
		[]float64{0.1, 0.2, 0.3, 0.4},
	)
	frame := equation.NewFeatureFrame(equation.CohortInputKeys, values)

	b.ReportAllocs()

	for b.Loop() {
		_, _ = cohort.Measure(frame)
	}
}
