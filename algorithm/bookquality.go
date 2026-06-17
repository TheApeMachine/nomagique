package algorithm

import (
	"math"

	"github.com/theapemachine/datura"
)

const bookQualityPayloadFields = 12

/*
BookQualityOutcome holds cancel/fill toxicity classification scores.
*/
type BookQualityOutcome struct {
	BluffScore   float64
	VacuumScore  float64
	SupportScore float64
	Strength     float64
	Category     int
	Eligible     bool
	Price        float64
}

/*
BookQuality classifies toxic bluff, liquidity vacuum, and hard support.

Payload layout: cancelBid, fillBid, cancelAsk, fillAsk, bidDepth, askDepth,
toxicNear, toxicBluffStrength, fillToCancelThreshold, churnGate, supportGate,
vacuumStrengthCap, lastPrice.
*/
type BookQuality struct {
	artifact *datura.Artifact
	outcome  BookQualityOutcome
}

/*
NewBookQuality returns a book-flow quality stage.
*/
func NewBookQuality() *BookQuality {
	return &BookQuality{
		artifact: datura.Acquire("bookquality", datura.Artifact_Type_json),
	}
}

func (bookQuality *BookQuality) Write(p []byte) (int, error) {
	return bookQuality.artifact.Write(p)
}

func (bookQuality *BookQuality) Read(p []byte) (int, error) {
	rehydrateArtifact(&bookQuality.artifact, "bookquality", datura.Artifact_Type_json)

	payload, err := bookQuality.artifact.Payload()

	if err == nil {
		bookQuality.outcome = bookQuality.evaluate(payloadSamples(payload))
		bookQuality.publishReadings()
	}

	return bookQuality.artifact.Read(p)
}

func (bookQuality *BookQuality) Close() error {
	return nil
}

/*
Outcome returns scores from the last Read.
*/
func (bookQuality *BookQuality) Outcome() BookQualityOutcome {
	return bookQuality.outcome
}

func (bookQuality *BookQuality) evaluate(batch []float64) BookQualityOutcome {
	if len(batch) < bookQualityPayloadFields+1 {
		return BookQualityOutcome{}
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
		return BookQualityOutcome{}
	}

	category, strength, bluffScore, vacuumScore, supportScore := classifyBookQuality(
		cancelBid, fillBid, cancelAsk, fillAsk,
		bidDepth, askDepth,
		toxicNear, toxicBluffStrength,
		threshold, churnGate, supportGate, vacuumStrengthCap,
	)

	if category == 0 || strength <= 0 {
		return BookQualityOutcome{}
	}

	if math.IsNaN(strength) || math.IsInf(strength, 0) {
		return BookQualityOutcome{}
	}

	evidence := math.Max(bluffScore, math.Max(vacuumScore, supportScore))

	if evidence <= 0 {
		return BookQualityOutcome{}
	}

	return BookQualityOutcome{
		BluffScore:   bluffScore,
		VacuumScore:  vacuumScore,
		SupportScore: supportScore,
		Strength:     strength,
		Category:     category,
		Eligible:     true,
		Price:        lastPrice,
	}
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
		supportScore = magnitudeMargin(depthBalance)

		return 3, depthBalance, 0, 0, supportScore
	}

	if threshold <= 0 {
		return 0, 0, 0, 0, 0
	}

	bidVacuum := bidRatio >= threshold && fillBid > 0
	askVacuum := askRatio >= threshold && fillAsk > 0

	if bidVacuum || askVacuum {
		margin := maxRatio - threshold
		vacuumScore = competitionMargin(margin, threshold)
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
		supportScore = competitionMargin(margin, half)
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
		return magnitudeMargin(churnRatio)
	}

	if churnRatio <= churnGate {
		return magnitudeMargin(churnRatio)
	}

	return competitionMargin(churnRatio-churnGate, churnGate)
}

func magnitudeMargin(value float64) float64 {
	if value <= 0 {
		return 0
	}

	return value / (1 + value)
}

func (bookQuality *BookQuality) publishReadings() {
	pokeFloat(bookQuality.artifact, "bookquality.bluff", bookQuality.outcome.BluffScore)
	pokeFloat(bookQuality.artifact, "bookquality.vacuum", bookQuality.outcome.VacuumScore)
	pokeFloat(bookQuality.artifact, "bookquality.support", bookQuality.outcome.SupportScore)
	pokeFloat(bookQuality.artifact, "bookquality.strength", bookQuality.outcome.Strength)
}

func (bookQuality *BookQuality) BluffReading() *BookQualityReading {
	return newBookQualityReading(bookQuality, func(outcome BookQualityOutcome) float64 {
		return outcome.BluffScore
	})
}

func (bookQuality *BookQuality) VacuumReading() *BookQualityReading {
	return newBookQualityReading(bookQuality, func(outcome BookQualityOutcome) float64 {
		return outcome.VacuumScore
	})
}

func (bookQuality *BookQuality) SupportReading() *BookQualityReading {
	return newBookQualityReading(bookQuality, func(outcome BookQualityOutcome) float64 {
		return outcome.SupportScore
	})
}

type BookQualityReading struct {
	artifact    *datura.Artifact
	bookQuality *BookQuality
	project     func(BookQualityOutcome) float64
}

func newBookQualityReading(
	bookQuality *BookQuality,
	project func(BookQualityOutcome) float64,
) *BookQualityReading {
	return &BookQualityReading{
		artifact:    datura.Acquire("bookquality-reading", datura.Artifact_Type_json),
		bookQuality: bookQuality,
		project:     project,
	}
}

func (reading *BookQualityReading) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	return len(p), nil
}

func (reading *BookQualityReading) Read(p []byte) (int, error) {
	value := 0.0

	if reading.bookQuality != nil && reading.project != nil {
		value = reading.project(reading.bookQuality.outcome)
	}

	_ = reading.artifact.SetPayload(encodePayload(value))

	return reading.artifact.Read(p)
}

func (reading *BookQualityReading) Close() error {
	return nil
}
