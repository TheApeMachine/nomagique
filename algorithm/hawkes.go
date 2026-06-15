package algorithm

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/hawkes"
	"gonum.org/v1/gonum/stat"
)

/*
Hawkes validates bivariate exponential-kernel parameters through empirical moments
composed from configured sample streams.
*/
type Hawkes struct {
	artifact *datura.Artifact
	params   hawkes.BivariateParams
	x        []float64
	y        []float64
	weights  []float64
	momentR  float64
	momentS  float64
	cross21R float64
	cross21S float64
	cross12R float64
	cross12S float64
}

/*
NewHawkes creates a Hawkes stage over configured x and y streams.
r and s select the mixed moment used by Read for fit diagnostics.
*/
func NewHawkes(
	params hawkes.BivariateParams,
	r, s float64,
	x, y, weights []float64,
) *Hawkes {
	return &Hawkes{
		artifact: datura.Acquire("hawkes", datura.Artifact_Type_json),
		params:   params,
		x:        x,
		y:        y,
		weights:  weights,
		momentR:  r,
		momentS:  s,
		cross21R: 2,
		cross21S: 1,
		cross12R: 1,
		cross12S: 2,
	}
}

func (hawkesProcess *Hawkes) Write(p []byte) (int, error) {
	return hawkesProcess.artifact.Write(p)
}

func (hawkesProcess *Hawkes) Read(p []byte) (int, error) {
	rehydrateArtifact(&hawkesProcess.artifact, "hawkes", datura.Artifact_Type_json)

	empirical := hawkesProcess.bivariateMoment(hawkesProcess.momentR, hawkesProcess.momentS)

	theoretical, ok := hawkes.TheoreticalCentralMoment(
		hawkesProcess.params, hawkesProcess.momentR, hawkesProcess.momentS,
	)

	if ok {
		confidence := hawkes.MomentConfidence(empirical, theoretical)
		out := encodePayload(confidence)
		_ = hawkesProcess.artifact.SetPayload(out)
	}

	return hawkesProcess.artifact.Read(p)
}

func (hawkesProcess *Hawkes) Close() error {
	return nil
}

/*
MethodOfMoments derives stable seed parameters from the configured streams.
*/
func (hawkesProcess *Hawkes) MethodOfMoments() (hawkes.BivariateParams, bool) {
	xValues, yValues, weights, ok := hawkesProcess.samples()

	if !ok {
		return hawkes.BivariateParams{}, false
	}

	return hawkes.MethodOfMoments(xValues, yValues, weights, hawkesProcess.params.Beta)
}

/*
CrossAsymmetry compares third-order mixed moments between the configured streams.
*/
func (hawkesProcess *Hawkes) CrossAsymmetry() float64 {
	moment21 := hawkesProcess.bivariateMoment(hawkesProcess.cross21R, hawkesProcess.cross21S)
	moment12 := hawkesProcess.bivariateMoment(hawkesProcess.cross12R, hawkesProcess.cross12S)

	return moment21 - moment12
}

/*
Reset clears derived state.
*/
func (hawkesProcess *Hawkes) Reset() error {
	hawkesProcess.weights = nil

	return nil
}

func (hawkesProcess *Hawkes) bivariateMoment(r, s float64) float64 {
	xValues, yValues, weights, ok := hawkesProcess.samples()

	if !ok {
		return 0
	}

	return stat.BivariateMoment(r, s, xValues, yValues, weights)
}

func (hawkesProcess *Hawkes) samples() (xValues, yValues, weights []float64, ok bool) {
	xValues = hawkesProcess.x
	yValues = hawkesProcess.y
	weights = hawkesProcess.weights

	if len(weights) == 0 {
		weights = nil
	}

	ok = len(xValues) == len(yValues) && len(xValues) >= 2

	if len(weights) != 0 && len(weights) != len(xValues) {
		ok = false
	}

	return xValues, yValues, weights, ok
}
