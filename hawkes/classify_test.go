package hawkes

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestClassifyFit(testingTB *testing.T) {
	readyGates := FitGates{SaturationRadius: 0.85, FrenzyAsymmetry: 0.3}
	notReadyGates := FitGates{}

	cases := []struct {
		name       string
		fit        BivariateFit
		asymmetry  float64
		preferY    bool
		gates      FitGates
		wantErr    bool
		wantCat    FitCategory
		wantConfGt float64
		wantConfEq float64
	}{
		{
			name:      "gates not ready",
			fit:       BivariateFit{SpectralRadius: 0.99, IntensityX: 2, MuX: 1},
			asymmetry: 0.9,
			gates:     notReadyGates,
			wantErr:   true,
		},
		{
			name: "saturation at spectral radius",
			fit: BivariateFit{
				MuX:            1,
				MuY:            1,
				IntensityX:     1.5,
				IntensityY:     1.5,
				SpectralRadius: 0.9,
			},
			asymmetry:  0.05,
			gates:      readyGates,
			wantCat:    FitCategorySaturation,
			wantConfGt: 0,
		},
		{
			name: "exhaustion below baseline",
			fit: BivariateFit{
				MuX:            2,
				MuY:            2,
				IntensityX:     0.5,
				IntensityY:     2,
				SpectralRadius: 0.4,
			},
			asymmetry:  0.05,
			gates:      readyGates,
			wantCat:    FitCategoryExhaustion,
			wantConfGt: 0,
		},
		{
			name: "frenzy asymmetry",
			fit: BivariateFit{
				MuX:            1,
				MuY:            1,
				IntensityX:     1.2,
				IntensityY:     1.2,
				SpectralRadius: 0.4,
			},
			asymmetry:  0.5,
			gates:      readyGates,
			wantCat:    FitCategoryFrenzy,
			wantConfGt: 0,
		},
		{
			name: "organic headroom",
			fit: BivariateFit{
				MuX:            1,
				MuY:            1,
				IntensityX:     1.1,
				IntensityY:     1.1,
				SpectralRadius: 0.5,
			},
			asymmetry:  0.1,
			gates:      readyGates,
			wantCat:    FitCategoryOrganic,
			wantConfGt: 0,
		},
		{
			name: "exhaustion on Y when preferY",
			fit: BivariateFit{
				MuX:            2,
				MuY:            2,
				IntensityX:     2,
				IntensityY:     0.4,
				SpectralRadius: 0.4,
			},
			asymmetry:  0.05,
			preferY:    true,
			gates:      readyGates,
			wantCat:    FitCategoryExhaustion,
			wantConfGt: 0,
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			category, confidence, err := ClassifyFit(
				testCase.fit,
				testCase.asymmetry,
				testCase.preferY,
				testCase.gates,
			)

			if testCase.wantErr {
				Convey("It should reject until gates are ready", func() {
					So(err, ShouldNotBeNil)
				})

				return
			}

			So(err, ShouldBeNil)

			Convey("It should classify as expected", func() {
				So(category, ShouldEqual, testCase.wantCat)

				if testCase.wantConfEq > 0 {
					So(confidence, ShouldEqual, testCase.wantConfEq)
				}

				if testCase.wantConfGt > 0 || testCase.wantConfEq == 0 {
					So(confidence, ShouldBeGreaterThan, testCase.wantConfGt)
				}
			})
		})
	}
}
