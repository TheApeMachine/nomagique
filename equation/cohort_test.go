package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
)

const cohortPayloadHeader = 6

func cohortBatch(
	window int,
	barSpacingSeconds float64,
	symbolReturns, marketReturns, peerCorrelations, peerEnergies []float64,
) []float64 {
	batch := make(
		[]float64,
		0,
		cohortPayloadHeader+len(symbolReturns)+len(marketReturns)+len(peerCorrelations)+len(peerEnergies),
	)
	batch = append(batch, float64(window))
	batch = append(batch,
		float64(len(symbolReturns)),
		float64(len(marketReturns)),
		float64(len(peerCorrelations)),
		float64(len(peerEnergies)),
	)
	batch = append(batch, barSpacingSeconds)
	batch = append(batch, symbolReturns...)
	batch = append(batch, marketReturns...)
	batch = append(batch, peerCorrelations...)
	batch = append(batch, peerEnergies...)

	return batch
}

func TestCohort_Read(testingTB *testing.T) {
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
				[]float64{0.01, 0.02, 0.01},
				[]float64{0.01, 0.02, 0.01},
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
				[]float64{0.5, 0.6, 0.7, 0.8},
				[]float64{0.4, 0.5, 0.6, 0.7},
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
			stage := equation.NewCohort(nil)
			writeErr := writeFeatureStage(stage, equation.CohortInputKeys, testCase.batch...)

			So(writeErr, ShouldBeNil)

			outbound, err := readStageOutput(stage)

			if !testCase.eligible {
				Convey("It should reject invalid payload", func() {
					So(err, ShouldNotBeNil)
				})

				return
			}

			So(err, ShouldBeNil)

			Convey("It should classify the cohort", func() {
				So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
				So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, testCase.wantCat)

				if testCase.wantCat == 1 {
					So(datura.Peek[float64](outbound, "output", "peakScore"), ShouldBeGreaterThan, 0)
				}
			})
		})
	}
}

func BenchmarkCohortRead(b *testing.B) {
	stage := equation.NewCohort(nil)
	values := cohortBatch(
		4,
		60,
		[]float64{0.5, 0.6, 0.7, 0.8},
		[]float64{0.4, 0.5, 0.6, 0.7},
		[]float64{0.1, 0.2, 0.3, 0.4},
		[]float64{0.1, 0.2, 0.3, 0.4},
	)

	b.ReportAllocs()

	for b.Loop() {
		_ = writeFeatureStage(stage, equation.CohortInputKeys, values...)
		frame := make([]byte, 4096)
		_, _ = stage.Read(frame)
	}
}
