package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/tests"
)

const cohortPayloadHeader = 5

func cohortBatch(
	window int,
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
	batch = append(batch, symbolReturns...)
	batch = append(batch, marketReturns...)
	batch = append(batch, peerCorrelations...)
	batch = append(batch, peerEnergies...)

	return batch
}

func TestCohort_evaluate(testingTB *testing.T) {
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
				[]float64{0.5, 0.6, 0.7, 0.8},
				[]float64{0.4, 0.5, 0.6, 0.7},
				[]float64{0.1, 0.2, 0.3, 0.4},
				[]float64{0.1, 0.2, 0.3, 0.4},
			),
			wantCat:  1,
			eligible: true,
		},
		{
			name: "stress negative correlation high energy",
			batch: cohortBatch(
				4,
				[]float64{-0.8, -0.7, -0.9, -0.85},
				[]float64{0.8, 0.7, 0.9, 0.85},
				[]float64{0.1, 0.2, -0.1, 0.05},
				[]float64{0.2, 0.3, 0.4, 0.5},
			),
			wantCat:  4,
			eligible: true,
		},
		{
			name: "mismatched segment length",
			batch: append(
				cohortBatch(3, []float64{1, 2}, []float64{1, 2}, []float64{0.1, 0.2}, []float64{0.1, 0.2}),
				99,
			),
			eligible: false,
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given cohort payload "+testCase.name, testingTB, func() {
			cohortStage := NewCohort()
			writeErr := tests.WriteSamples(cohortStage, testCase.batch...)

			So(writeErr, ShouldBeNil)

			frame := make([]byte, 4096)
			_, _ = cohortStage.Read(frame)
			outbound := datura.Acquire("test-out", datura.APPJSON)
			_, _ = outbound.Write(frame)

			if !testCase.eligible {
				Convey("It should reject invalid payload", func() {
					So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 0)
				})

				return
			}

			Convey("It should classify the cohort", func() {
				So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
				So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, testCase.wantCat)
			})
		})
	}
}

func TestCohort_encodePayloadRoundTrip(testingTB *testing.T) {
	Convey("Given encoded cohort header via encodePayload", testingTB, func() {
		payload := encodePayload(3, 3, 3, 4, 4)
		samples := payloadSamples(payload)

		Convey("It should decode header fields", func() {
			So(len(samples), ShouldEqual, 5)
			So(int(samples[0]), ShouldEqual, 3)
			So(int(samples[1]), ShouldEqual, 3)
			So(int(samples[4]), ShouldEqual, 4)
		})
	})
}
