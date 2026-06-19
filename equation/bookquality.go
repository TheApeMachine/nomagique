package equation

import (
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
func NewBookQuality() *BookQuality {
	return &BookQuality{
		artifact: datura.Acquire("bookquality", datura.APPJSON).RetainStageAttributes(),
	}
}

func (bookQuality *BookQuality) StageArtifact() *datura.Artifact {
	return bookQuality.artifact
}

func (bookQuality *BookQuality) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](bookQuality.artifact, "output") == nil

	bookQuality.artifact.Clear("sample")

	n, err := bookQuality.artifact.Write(p)

	if bootstrap {
		bookQuality.artifact.Clear("output")
	}

	return n, err
}

func (bookQuality *BookQuality) Read(p []byte) (int, error) {
	batch := FloatBatch(bookQuality.artifact)

	if len(batch) < bookQualityPayloadFields+1 {
		bookQuality.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return bookQuality.artifact.Read(p)
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
		bookQuality.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return bookQuality.artifact.Read(p)
	}

	category, strength, bluffScore, vacuumScore, supportScore := classifyBookQuality(
		cancelBid, fillBid, cancelAsk, fillAsk,
		bidDepth, askDepth,
		toxicNear, toxicBluffStrength,
		threshold, churnGate, supportGate, vacuumStrengthCap,
	)

	if category == 0 || strength <= 0 {
		bookQuality.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return bookQuality.artifact.Read(p)
	}

	if math.IsNaN(strength) || math.IsInf(strength, 0) {
		bookQuality.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return bookQuality.artifact.Read(p)
	}

	evidence := math.Max(bluffScore, math.Max(vacuumScore, supportScore))

	if evidence <= 0 {
		bookQuality.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return bookQuality.artifact.Read(p)
	}

	bookQuality.artifact.Poke(datura.Map[float64]{
		"value":        strength,
		"bluffScore":   bluffScore,
		"vacuumScore":  vacuumScore,
		"supportScore": supportScore,
		"strength":     strength,
		"category":     float64(category),
		"price":        lastPrice,
	}, "output")

	return bookQuality.artifact.Read(p)
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
) (category int, strength, bluffScore, vacuumScore, supportScore float64) {
	if toxicNear && churnGate > 0 {
		bluffScore = toxicBluffEvidence(toxicBluffStrength, churnGate)

		return 1, toxicBluffStrength, bluffScore, 0, 0
	}

	bidRatio := CancelFillRatio(cancelBid, fillBid)
	askRatio := CancelFillRatio(cancelAsk, fillAsk)
	maxRatio := math.Max(bidRatio, askRatio)

	if bidDepth > 0 && askDepth > 0 && maxRatio == 0 && (fillBid > 0 || fillAsk > 0) {
		depthBalance := math.Min(bidDepth, askDepth) / math.Max(bidDepth, askDepth)
		supportScore = probability.MagnitudeMargin(depthBalance)

		return 3, depthBalance, 0, 0, supportScore
	}

	if threshold <= 0 {
		return 0, 0, 0, 0, 0
	}

	bidVacuum := bidRatio >= threshold && fillBid > 0
	askVacuum := askRatio >= threshold && fillAsk > 0

	if bidVacuum || askVacuum {
		margin := maxRatio - threshold
		vacuumScore = probability.CompetitionMargin(margin, threshold)
		strengthCap := vacuumStrengthCap

		if strengthCap <= 0 {
			strengthCap = math.Max(2, maxRatio/threshold)
		}

		strength = math.Min(maxRatio/threshold, strengthCap)

		return 2, strength, 0, vacuumScore, 0
	}

	if supportGate <= 0 && bidRatio > 0 && askRatio > 0 {
		supportGate = math.Min(bidRatio, askRatio)
	}

	if bidRatio > 0 && askRatio > 0 && bidRatio < supportGate && askRatio < supportGate {
		half := supportGate
		margin := half - maxRatio
		supportScore = probability.CompetitionMargin(margin, half)
		strength = supportScore

		return 3, strength, 0, 0, supportScore
	}

	return 0, 0, 0, 0, 0
}

func toxicBluffEvidence(churnRatio, churnGate float64) float64 {
	if churnRatio <= 0 {
		return 0
	}

	if churnGate <= 0 {
		return probability.MagnitudeMargin(churnRatio)
	}

	if churnRatio <= churnGate {
		return probability.MagnitudeMargin(churnRatio)
	}

	return probability.CompetitionMargin(churnRatio-churnGate, churnGate)
}
