package hawkes

import (
	"encoding/binary"
	"math"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
	"gonum.org/v1/gonum/stat"
)

/*
Moment validates bivariate exponential-kernel parameters through empirical moments.
*/
type Moment struct {
	artifact *datura.Artifact
	params   BivariateParams
	momentR  float64
	momentS  float64
}

/*
NewMoment creates a Hawkes moment-confidence stage.
momentR and momentS select the mixed moment used for fit diagnostics.
*/
func NewMoment(params BivariateParams, momentR, momentS float64) *Moment {
	return &Moment{
		artifact: datura.Acquire("hawkes-moment", datura.APPJSON),
		params:   params,
		momentR:  momentR,
		momentS:  momentS,
	}
}

func (moment *Moment) Write(p []byte) (int, error) {
	moment.artifact.WithPayload(p)
	return len(p), nil
}

func (moment *Moment) Read(p []byte) (int, error) {
	state := datura.Acquire("hawkes-moment-state", datura.APPJSON)

	if _, err := state.Write(moment.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	xValues, yValues, weights, ok := momentSamples(state, moment.artifact)

	if !ok {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes-moment: require aligned sample streams of at least two observations",
			nil,
		))
	}

	empirical := stat.BivariateMoment(moment.momentR, moment.momentS, xValues, yValues, weights)
	theoretical, theoreticalOK := TheoreticalCentralMoment(moment.params, moment.momentR, moment.momentS)
	confidence := 0.0

	if theoreticalOK {
		var confidenceOK bool

		confidence, confidenceOK = MomentConfidence(empirical, theoretical)

		if !confidenceOK {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"hawkes-moment: confidence could not be derived",
				nil,
			))
		}
	}

	state.MergeOutput("value", confidence)
	state.MergeOutput("empirical", empirical)
	state.MergeOutput("theoretical", theoretical)
	state.MergeOutput("confidence", confidence)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value", "empirical", "theoretical", "confidence"})

	return state.Read(p)
}

func (moment *Moment) Close() error {
	return nil
}

/*
Fit estimates bivariate Hawkes parameters from timestamp arrival streams.
*/
type Fit struct {
	artifact  *datura.Artifact
	horizon   int64
	prior     BivariateFit
	estimator *BivariateEstimator
}

/*
NewFit creates a timestamp-stream Hawkes fit stage.
horizonUnixNano is the observation horizon in Unix nanoseconds.
*/
func NewFit(horizonUnixNano int64, prior BivariateFit) *Fit {
	return &Fit{
		artifact:  datura.Acquire("hawkes-fit", datura.APPJSON),
		horizon:   horizonUnixNano,
		prior:     prior,
		estimator: NewBivariateEstimator(prior),
	}
}

func (fit *Fit) Write(p []byte) (int, error) {
	fit.artifact.WithPayload(p)
	return len(p), nil
}

func (fit *Fit) Read(p []byte) (int, error) {
	state := datura.Acquire("hawkes-fit-state", datura.APPJSON)

	if _, err := state.Write(fit.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	xTimes, yTimes, ok := fitTimes(state, fit.artifact)

	if !ok {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes-fit: require aligned arrival timestamp streams",
			nil,
		))
	}

	stream := NewArrivalStream(fitTimesToTime(xTimes), fitTimesToTime(yTimes))
	horizon := fitHorizon(fit)
	fitted := fit.estimator.Fit(stream, horizon)

	if !fitted.Valid() {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes-fit: fit did not converge to valid parameters",
			nil,
		))
	}

	asymmetry := fitted.Asymmetry(false)
	ratio := 0.0

	if asymmetry > 0 && fitted.MuX > 0 {
		ratio = fitted.IntensityX / fitted.MuX
	}

	if asymmetry <= 0 && fitted.MuY > 0 {
		ratio = fitted.IntensityY / fitted.MuY
	}

	state.MergeOutput("value", ratio)
	state.MergeOutput("excitationRatio", ratio)
	state.MergeOutput("spectralRadius", fitted.SpectralRadius)
	state.MergeOutput("asymmetry", asymmetry)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value", "excitationRatio", "spectralRadius", "asymmetry"})

	return state.Read(p)
}

func (fit *Fit) Close() error {
	return nil
}

func momentSamples(wire, config *datura.Artifact) (xValues, yValues, weights []float64, ok bool) {
	batch := equation.Features(wire)

	if len(batch) == 0 {
		batch = fitFloatBatch(wire)
	}

	if len(batch) == 0 {
		batch = datura.Peek[[]float64](wire, "batch")
	}

	if len(batch) < 4 || len(batch)%2 != 0 {
		return nil, nil, nil, false
	}

	half := len(batch) / 2
	xValues = batch[:half]
	yValues = batch[half:]
	weights = datura.Peek[[]float64](wire, "config", "weights")

	if len(weights) == 0 {
		weights = datura.Peek[[]float64](config, "config", "weights")
	}

	if len(weights) == 0 {
		weights = nil
	}

	ok = len(xValues) == len(yValues) && len(xValues) >= 2

	if len(weights) != 0 && len(weights) != len(xValues) {
		ok = false
	}

	return xValues, yValues, weights, ok
}

func fitTimes(wire, config *datura.Artifact) (xTimes, yTimes []float64, ok bool) {
	xCount := int(datura.Peek[float64](wire, "config", "xCount"))
	yCount := int(datura.Peek[float64](wire, "config", "yCount"))

	if xCount <= 0 {
		xCount = int(datura.Peek[float64](config, "config", "xCount"))
	}

	if yCount <= 0 {
		yCount = int(datura.Peek[float64](config, "config", "yCount"))
	}

	batch := equation.Features(wire)

	if len(batch) == 0 {
		batch = fitFloatBatch(wire)
	}

	if len(batch) == 0 {
		batch = datura.Peek[[]float64](wire, "batch")
	}

	if xCount <= 0 || yCount <= 0 || len(batch) < xCount+yCount {
		return nil, nil, false
	}

	xTimes = batch[:xCount]
	yTimes = batch[xCount : xCount+yCount]

	for _, sample := range append(xTimes, yTimes...) {
		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return nil, nil, false
		}
	}

	return xTimes, yTimes, len(xTimes)+len(yTimes) >= 2
}

func fitFloatBatch(artifact *datura.Artifact) []float64 {
	if !artifact.HasEncryptedPayload() {
		return nil
	}

	payload := artifact.DecryptPayload()

	if len(payload) == 0 || len(payload)%8 != 0 {
		return nil
	}

	samples := make([]float64, len(payload)/8)

	for index := range samples {
		offset := index * 8
		value := math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))

		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil
		}

		samples[index] = value
	}

	return samples
}

func fitHorizon(fit *Fit) time.Time {
	horizonNano := fit.horizon

	if horizonNano <= 0 {
		horizonNano = int64(datura.Peek[float64](fit.artifact, "config", "horizonUnixNano"))
	}

	return time.Unix(0, horizonNano)
}

func fitTimesToTime(samples []float64) []time.Time {
	times := make([]time.Time, len(samples))

	for index, sample := range samples {
		times[index] = time.Unix(0, int64(sample))
	}

	return times
}

func EncodeMomentBatch(xStream, yStream []float64) []float64 {
	if len(xStream) != len(yStream) || len(xStream) < 2 {
		return nil
	}

	batch := make([]float64, 0, len(xStream)+len(yStream))
	batch = append(batch, xStream...)
	batch = append(batch, yStream...)

	return batch
}

func EncodeFitBatch(xTimes, yTimes []float64) []float64 {
	if len(xTimes)+len(yTimes) < 2 {
		return nil
	}

	batch := make([]float64, 0, len(xTimes)+len(yTimes))
	batch = append(batch, xTimes...)
	batch = append(batch, yTimes...)

	return batch
}
