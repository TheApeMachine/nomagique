package hawkes

import (
	"fmt"

	"github.com/theapemachine/nomagique/probability"
)

/*
FitCategory names the dominant Hawkes regime for a fitted process.
*/
type FitCategory int

const (
	FitCategoryOrganic FitCategory = iota
	FitCategoryFrenzy
	FitCategorySaturation
	FitCategoryExhaustion
)

const uniformFitConfidence = 1.0 / 4.0

/*
ClassifyFit maps a fit and asymmetry to a category and confidence score.
Classification is withheld until fit gates are ready.
*/
func ClassifyFit(
	fit BivariateFit,
	asymmetry float64,
	preferY bool,
	gates FitGates,
) (category FitCategory, confidence float64, err error) {
	if !gates.Ready() {
		return FitCategoryOrganic, 0, fmt.Errorf("hawkes: fit gates are not ready")
	}

	return classifyFitWithGates(fit, asymmetry, preferY, gates)
}

func classifyFitWithGates(
	fit BivariateFit,
	asymmetry float64,
	preferY bool,
	gates FitGates,
) (category FitCategory, confidence float64, err error) {
	saturationRadius := gates.SaturationRadius
	frenzyAsymmetry := gates.FrenzyAsymmetry
	intensity, baseline := fit.IntensityX, fit.MuX

	if preferY {
		intensity, baseline = fit.IntensityY, fit.MuY
	}

	switch {
	case asymmetry > frenzyAsymmetry:
		margin := asymmetry - frenzyAsymmetry
		span := 1 - frenzyAsymmetry

		if margin <= 0 || span <= 0 {
			return FitCategoryFrenzy, uniformFitConfidence, nil
		}

		confidence, err = probability.CompetitionMargin(margin, span)

		if err != nil {
			return FitCategoryOrganic, 0, err
		}

		return FitCategoryFrenzy, confidence, nil
	case fit.SpectralRadius >= saturationRadius:
		margin := fit.SpectralRadius - saturationRadius
		span := 1 - saturationRadius

		if margin <= 0 || span <= 0 {
			return FitCategorySaturation, uniformFitConfidence, nil
		}

		confidence, err = probability.CompetitionMargin(margin, span)

		if err != nil {
			return FitCategoryOrganic, 0, err
		}

		return FitCategorySaturation, confidence, nil
	case baseline > 0 && intensity < baseline:
		margin := baseline - intensity

		if margin <= 0 {
			return FitCategoryExhaustion, uniformFitConfidence, nil
		}

		confidence, err = probability.CompetitionMargin(margin, baseline)

		if err != nil {
			return FitCategoryOrganic, 0, err
		}

		return FitCategoryExhaustion, confidence, nil
	default:
		headroom := -1.0

		if fit.SpectralRadius < saturationRadius {
			margin := saturationRadius - fit.SpectralRadius
			saturationHead, err := probability.CompetitionMargin(margin, saturationRadius)

			if err != nil {
				return FitCategoryOrganic, 0, err
			}

			if saturationHead > headroom {
				headroom = saturationHead
			}
		}

		if baseline > 0 && intensity >= baseline && asymmetry < frenzyAsymmetry {
			margin := intensity - baseline
			organicHead := margin / (margin + baseline)

			if organicHead > headroom {
				headroom = organicHead
			}
		}

		if asymmetry < frenzyAsymmetry {
			margin := frenzyAsymmetry - asymmetry
			frenzyHead, err := probability.CompetitionMargin(margin, frenzyAsymmetry)

			if err != nil {
				return FitCategoryOrganic, 0, err
			}

			if frenzyHead > headroom {
				headroom = frenzyHead
			}
		}

		if headroom < 0 {
			return FitCategoryOrganic, uniformFitConfidence, nil
		}

		return FitCategoryOrganic, headroom, nil
	}
}
