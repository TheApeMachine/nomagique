package equation

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/learning"
)

/*
Verticality scores volume lift, price precursor, and spread compression for
downstream classification.
*/
type Verticality struct {
	artifact *datura.Artifact
}

/*
NewVerticality returns a verticality stage with balanced weights on the artifact.
*/
func NewVerticality() *Verticality {
	verticality := &Verticality{
		artifact: datura.Acquire("verticality", datura.APPJSON),
	}

	weights, err := learning.NewClassifierWeights(1.0, learning.ClassifierFeatureScales{
		RVol:        1,
		Precursor:   1,
		Compression: 1,
	})

	if err != nil {
		weights = equalClassifierWeights()
	}

	verticality.artifact.Poke(weights.WIgnVol, "weights", "wIgnVol")
	verticality.artifact.Poke(weights.WIgnPrec, "weights", "wIgnPrec")
	verticality.artifact.Poke(weights.WCoilComp, "weights", "wCoilComp")
	verticality.artifact.Poke(weights.WCoilPrec, "weights", "wCoilPrec")
	verticality.artifact.Poke(weights.WOrgPrec, "weights", "wOrgPrec")
	verticality.artifact.Poke(weights.WOrgComp, "weights", "wOrgComp")
	verticality.artifact.Poke(weights.WOrgVol, "weights", "wOrgVol")
	verticality.artifact.Poke(weights.WExVol, "weights", "wExVol")
	verticality.artifact.Poke(weights.WExPrec, "weights", "wExPrec")

	return verticality
}

func (verticality *Verticality) Write(p []byte) (int, error) {
	weights := datura.Peek[datura.Map[float64]](verticality.artifact, "weights")

	n, err := verticality.artifact.Write(p)

	if weights != nil {
		verticality.artifact.Poke(weights, "weights")
	}

	return n, err
}

func (verticality *Verticality) Read(p []byte) (int, error) {
	features := datura.Peek[[]float64](verticality.artifact, "features")

	if len(features) < 6 {
		return verticality.artifact.Read(p)
	}

	volume := features[0]
	vwap := features[1]
	last := features[2]
	bid := features[3]
	ask := features[4]
	_ = features[5]

	mid := (bid + ask) / 2

	spread := 0.0

	if mid > 0 {
		spread = (ask - bid) / mid
	}

	rvol := 0.0

	if vwap > 0 {
		rvol = volume / vwap
	}

	precursor := 0.0

	if vwap > 0 {
		precursor = math.Abs(last-vwap) / vwap
	}

	compression := 0.0

	if spread > 0 {
		compression = 1.0 / spread
	}

	rvolScore := boundedFeatureScore(rvol, 1)
	precursorScore := boundedFeatureScore(precursor, 0)
	compressionScore := boundedFeatureScore(compression, 1)

	scores := []float64{
		rvolScore*datura.Peek[float64](verticality.artifact, "weights", "wIgnVol") +
			precursorScore*datura.Peek[float64](verticality.artifact, "weights", "wIgnPrec"),
		compressionScore*datura.Peek[float64](verticality.artifact, "weights", "wCoilComp") +
			(1.0-precursorScore)*datura.Peek[float64](verticality.artifact, "weights", "wCoilPrec"),
		precursorScore*datura.Peek[float64](verticality.artifact, "weights", "wOrgPrec") +
			(1.0-compressionScore)*datura.Peek[float64](verticality.artifact, "weights", "wOrgComp") +
			rvolScore*datura.Peek[float64](verticality.artifact, "weights", "wOrgVol"),
		(1.0-rvolScore)*datura.Peek[float64](verticality.artifact, "weights", "wExVol") +
			(1.0-precursorScore)*datura.Peek[float64](verticality.artifact, "weights", "wExPrec"),
	}

	verticality.artifact.Poke(scores[0], "output", "ignition")
	verticality.artifact.Poke(scores[1], "output", "compression")
	verticality.artifact.Poke(scores[2], "output", "trend")
	verticality.artifact.Poke(scores[3], "output", "exhaustion")
	verticality.artifact.Poke(scores[0], "output", "value")

	return verticality.artifact.Read(p)
}

func (verticality *Verticality) Close() error {
	return nil
}

func boundedFeatureScore(value, floor float64) float64 {
	if value <= floor {
		return 0
	}

	return value / (1 + value)
}

func equalClassifierWeights() learning.ClassifierWeights {
	return learning.ClassifierWeights{
		Threshold: 1.0,
		WIgnVol:   0.5,
		WIgnPrec:  0.5,
		WCoilComp: 0.5,
		WCoilPrec: 0.5,
		WOrgPrec:  1.0 / 3.0,
		WOrgComp:  1.0 / 3.0,
		WOrgVol:   1.0 / 3.0,
		WExVol:    0.5,
		WExPrec:   0.5,
	}
}
