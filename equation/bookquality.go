package equation

import (
	"errors"
	"fmt"
	"math"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/probability"
)

var errNoBookQualityCategory = errors.New("no book quality category matched")

/*
BookQuality classifies toxic bluff, liquidity vacuum, and hard support.
*/
type BookQuality struct{}

/*
BookQualityInput contains the float-only book quality inputs.
*/
type BookQualityInput struct {
	CancelBid          float64
	FillBid            float64
	CancelAsk          float64
	FillAsk            float64
	BidDepth           float64
	AskDepth           float64
	ToxicNear          bool
	ToxicBluffStrength float64
	Threshold          float64
	ChurnGate          float64
	SupportGate        float64
	VacuumStrengthCap  float64
	LastPrice          float64
}

/*
BookQualityOutput contains the float-only book quality scores.
*/
type BookQualityOutput struct {
	Value        float64
	BluffScore   float64
	VacuumScore  float64
	SupportScore float64
	Strength     float64
	Category     float64
	Price        float64
}

/*
NewBookQuality returns a book-flow quality calculator.
*/
func NewBookQuality() *BookQuality {
	return &BookQuality{}
}

/*
Measure calculates book-quality scores from floats without artifact transport.
*/
func (bookQuality *BookQuality) Measure(
	input BookQualityInput,
) (BookQualityOutput, error) {
	if input.LastPrice <= 0 {
		return BookQualityOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"bookquality: last price must be positive",
			nil,
		))
	}

	category, strength, bluffScore, vacuumScore, supportScore, err := classifyBookQuality(
		input.CancelBid, input.FillBid, input.CancelAsk, input.FillAsk,
		input.BidDepth, input.AskDepth,
		input.ToxicNear, input.ToxicBluffStrength,
		input.Threshold, input.ChurnGate, input.SupportGate, input.VacuumStrengthCap,
	)

	if err != nil {
		if errors.Is(err, errNoBookQualityCategory) {
			return bookQuality.neutral(input.LastPrice), nil
		}

		return BookQualityOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("bookquality: %v", err),
			err,
		))
	}

	if category == 0 || strength <= 0 {
		return bookQuality.neutral(input.LastPrice), nil
	}

	if math.IsNaN(strength) || math.IsInf(strength, 0) {
		return BookQualityOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"bookquality: strength is non-finite",
			nil,
		))
	}

	evidence := math.Max(bluffScore, math.Max(vacuumScore, supportScore))

	if evidence <= 0 {
		return BookQualityOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"bookquality: no positive evidence",
			nil,
		))
	}

	return BookQualityOutput{
		Value:        strength,
		BluffScore:   bluffScore,
		VacuumScore:  vacuumScore,
		SupportScore: supportScore,
		Strength:     strength,
		Category:     float64(category),
		Price:        input.LastPrice,
	}, nil
}

func (bookQuality *BookQuality) neutral(lastPrice float64) BookQualityOutput {
	return BookQualityOutput{
		Price: lastPrice,
	}
}

func classifyBookQuality(
	cancelBid, fillBid, cancelAsk, fillAsk float64,
	bidDepth, askDepth float64,
	toxicNear bool,
	toxicBluffStrength float64,
	threshold, churnGate, supportGate, vacuumStrengthCap float64,
) (category int, strength, bluffScore, vacuumScore, supportScore float64, err error) {
	if toxicNear && toxicBluffStrength > 0 {
		bluffScore, err = toxicBluffEvidence(toxicBluffStrength, churnGate)

		if err != nil {
			return 0, 0, 0, 0, 0, err
		}

		return 1, toxicBluffStrength, bluffScore, 0, 0, nil
	}

	bidRatio := 0.0

	if cancelBid > 0 && fillBid > 0 {
		bidRatio, err = CancelFillRatio(cancelBid, fillBid)

		if err != nil {
			return 0, 0, 0, 0, 0, err
		}
	}

	if cancelBid > 0 && fillBid <= 0 {
		return 0, 0, 0, 0, 0, errNoBookQualityCategory
	}

	askRatio := 0.0

	if cancelAsk > 0 && fillAsk > 0 {
		askRatio, err = CancelFillRatio(cancelAsk, fillAsk)

		if err != nil {
			return 0, 0, 0, 0, 0, err
		}
	}

	if cancelAsk > 0 && fillAsk <= 0 {
		return 0, 0, 0, 0, 0, errNoBookQualityCategory
	}

	maxRatio := math.Max(bidRatio, askRatio)

	if bidDepth > 0 && askDepth > 0 && maxRatio == 0 && (fillBid > 0 || fillAsk > 0) {
		depthBalance := math.Min(bidDepth, askDepth) / math.Max(bidDepth, askDepth)
		supportScore, err = probability.MagnitudeMargin(depthBalance)

		if err != nil {
			return 0, 0, 0, 0, 0, err
		}

		return 3, depthBalance, 0, 0, supportScore, nil
	}

	if threshold <= 0 {
		return 0, 0, 0, 0, 0, errNoBookQualityCategory
	}

	bidVacuum := bidRatio > threshold && fillBid > 0
	askVacuum := askRatio > threshold && fillAsk > 0

	if bidVacuum || askVacuum {
		margin := maxRatio - threshold

		if margin <= 0 {
			return 0, 0, 0, 0, 0, fmt.Errorf("bookquality: vacuum margin is not positive")
		}

		vacuumScore, err = probability.CompetitionMargin(margin, threshold)

		if err != nil {
			return 0, 0, 0, 0, 0, err
		}

		strength = vacuumScore

		if vacuumStrengthCap > 0 {
			strength = math.Min(strength, vacuumStrengthCap)
		}

		return 2, strength, 0, vacuumScore, 0, nil
	}

	if supportGate <= 0 && bidRatio > 0 && askRatio > 0 {
		supportGate = math.Min(bidRatio, askRatio)
	}

	if bidRatio > 0 && askRatio > 0 && bidRatio < supportGate && askRatio < supportGate {
		half := supportGate
		margin := half - maxRatio

		if margin <= 0 {
			return 0, 0, 0, 0, 0, fmt.Errorf("bookquality: support margin is not positive")
		}

		supportScore, err = probability.CompetitionMargin(margin, half)

		if err != nil {
			return 0, 0, 0, 0, 0, err
		}

		return 3, supportScore, 0, 0, supportScore, nil
	}

	return 0, 0, 0, 0, 0, errNoBookQualityCategory
}

func toxicBluffEvidence(churnRatio, churnGate float64) (float64, error) {
	if churnRatio <= 0 {
		return 0, fmt.Errorf("churn ratio must be positive")
	}

	if churnGate <= 0 {
		return probability.MagnitudeMargin(churnRatio)
	}

	if churnRatio <= churnGate {
		return probability.MagnitudeMargin(churnRatio)
	}

	return probability.CompetitionMargin(churnRatio-churnGate, churnGate)
}
