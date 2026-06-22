package equation

import (
	"fmt"
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/probability"
)

const bookQualityPayloadFields = 12

/*
BookQuality classifies toxic bluff, liquidity vacuum, and hard support.

Payload layout: cancelBid, fillBid, cancelAsk, fillAsk, bidDepth, askDepth,
toxicNear, toxicBluffStrength, fillToCancelThreshold, churnGate, supportGate,
vacuumStrengthCap, lastPrice.
*/
type BookQuality struct {
	artifact *datura.Artifact
}

/*
NewBookQuality returns a book-flow quality stage.
*/
func NewBookQuality() io.ReadWriteCloser {
	return &BookQuality{
		artifact: datura.Acquire("book-quality", datura.APPJSON),
	}
}

func (bookQuality *BookQuality) Write(p []byte) (int, error) {
	bookQuality.artifact.WithPayload(p)
	return len(p), nil
}

func (bookQuality *BookQuality) Read(p []byte) (int, error) {
	state, err := stageState(bookQuality.artifact.DecryptPayload())

	if err != nil {
		return 0, err
	}

	batch := Features(state)

	if len(batch) < bookQualityPayloadFields+1 {
		return rejectStage(state, "bookquality: insufficient payload")
	}

	cancelBid := batch[0]
	fillBid := batch[1]
	cancelAsk := batch[2]
	fillAsk := batch[3]
	bidDepth := batch[4]
	askDepth := batch[5]
	toxicNear := batch[6] > 0
	toxicBluffStrength := batch[7]
	threshold := batch[8]
	churnGate := batch[9]
	supportGate := batch[10]
	vacuumStrengthCap := batch[11]
	lastPrice := batch[12]

	if lastPrice <= 0 {
		return rejectStage(state, "bookquality: lastPrice must be positive")
	}

	category, strength, bluffScore, vacuumScore, supportScore, err := classifyBookQuality(
		cancelBid, fillBid, cancelAsk, fillAsk,
		bidDepth, askDepth,
		toxicNear, toxicBluffStrength,
		threshold, churnGate, supportGate, vacuumStrengthCap,
	)

	if err != nil {
		return rejectStage(state, fmt.Sprintf("bookquality: %v", err))
	}

	if category == 0 || strength <= 0 {
		return rejectStage(state, "bookquality: no qualifying category")
	}

	if math.IsNaN(strength) || math.IsInf(strength, 0) {
		return rejectStage(state, "bookquality: strength is non-finite")
	}

	evidence := math.Max(bluffScore, math.Max(vacuumScore, supportScore))

	if evidence <= 0 {
		return rejectStage(state, "bookquality: no positive evidence")
	}

	return emitOutput(state, p, datura.Map[float64]{
		"value":        strength,
		"bluffScore":   bluffScore,
		"vacuumScore":  vacuumScore,
		"supportScore": supportScore,
		"strength":     strength,
		"category":     float64(category),
		"price":        lastPrice,
	})
}

func (bookQuality *BookQuality) Close() error {
	return nil
}

func classifyBookQuality(
	cancelBid, fillBid, cancelAsk, fillAsk float64,
	bidDepth, askDepth float64,
	toxicNear bool,
	toxicBluffStrength float64,
	threshold, churnGate, supportGate, vacuumStrengthCap float64,
) (category int, strength, bluffScore, vacuumScore, supportScore float64, err error) {
	if toxicNear && churnGate > 0 {
		bluffScore, err = toxicBluffEvidence(toxicBluffStrength, churnGate)

		if err != nil {
			return 0, 0, 0, 0, 0, err
		}

		return 1, toxicBluffStrength, bluffScore, 0, 0, nil
	}

	bidRatio := CancelFillRatio(cancelBid, fillBid)
	askRatio := CancelFillRatio(cancelAsk, fillAsk)
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
		if bidRatio > 0 || askRatio > 0 {
			return 0, 0, 0, 0, 0, fmt.Errorf("bookquality: fillToCancelThreshold not ready")
		}

		return 0, 0, 0, 0, 0, fmt.Errorf("bookquality: no qualifying category")
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

		strengthCap := vacuumStrengthCap

		if strengthCap <= 0 {
			strengthCap = maxRatio / threshold
		}

		strength = math.Min(maxRatio/threshold, strengthCap)

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

	return 0, 0, 0, 0, 0, fmt.Errorf("no book quality category matched")
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
