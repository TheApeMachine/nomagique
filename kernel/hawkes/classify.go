package hawkes

import "github.com/theapemachine/nomagique/probability"

const (
	saturationRadius = 0.85
	frenzyAsymmetry  = 0.15
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
*/
func ClassifyFit(
	fit BivariateFit,
	asymmetry float64,
	preferY bool,
) (category FitCategory, confidence float64) {
	intensity, baseline := fit.IntensityX, fit.MuX

	if preferY {
		intensity, baseline = fit.IntensityY, fit.MuY
	}

	switch {
	case fit.SpectralRadius >= saturationRadius:
		margin := fit.SpectralRadius - saturationRadius
		span := 1 - saturationRadius

		if margin <= 0 || span <= 0 {
			return FitCategorySaturation, uniformFitConfidence
		}

		return FitCategorySaturation, probability.CompetitionMargin(margin, span)
	case baseline > 0 && intensity < baseline:
		margin := baseline - intensity

		if margin <= 0 {
			return FitCategoryExhaustion, uniformFitConfidence
		}

		return FitCategoryExhaustion, probability.CompetitionMargin(margin, baseline)
	case asymmetry >= frenzyAsymmetry:
		margin := asymmetry - frenzyAsymmetry
		span := 1 - frenzyAsymmetry

		if margin <= 0 || span <= 0 {
			return FitCategoryFrenzy, uniformFitConfidence
		}

		return FitCategoryFrenzy, probability.CompetitionMargin(margin, span)
	default:
		headroom := -1.0

		if fit.SpectralRadius < saturationRadius {
			margin := saturationRadius - fit.SpectralRadius
			saturationHead := probability.CompetitionMargin(margin, saturationRadius)

			if saturationHead > headroom {
				headroom = saturationHead
			}
		}

		if baseline > 0 && intensity >= baseline {
			margin := intensity - baseline
			organicHead := margin / (margin + baseline)

			if organicHead > headroom {
				headroom = organicHead
			}
		}

		if asymmetry < frenzyAsymmetry {
			margin := frenzyAsymmetry - asymmetry
			frenzyHead := probability.CompetitionMargin(margin, frenzyAsymmetry)

			if frenzyHead > headroom {
				headroom = frenzyHead
			}
		}

		if headroom < 0 {
			return FitCategoryOrganic, uniformFitConfidence
		}

		return FitCategoryOrganic, headroom
	}
}
