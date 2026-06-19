package hawkes

import (
	"math"
	"time"

	"github.com/theapemachine/datura"
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
		artifact: datura.Acquire("hawkes-moment", datura.APPJSON).RetainStageAttributes(),
		params:   params,
		momentR:  momentR,
		momentS:  momentS,
	}
}

func (moment *Moment) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](moment.artifact, "output") == nil

	moment.artifact.Clear("sample")
	moment.artifact.Clear("batch")

	n, err := moment.artifact.Write(p)

	if bootstrap {
		moment.artifact.Clear("output")
	}

	return n, err
}

func (moment *Moment) Read(p []byte) (int, error) {
	xValues, yValues, weights, ok := momentSamples(moment.artifact)

	if !ok {
		moment.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return moment.artifact.Read(p)
	}

	empirical := stat.BivariateMoment(moment.momentR, moment.momentS, xValues, yValues, weights)
	theoretical, theoreticalOK := TheoreticalCentralMoment(moment.params, moment.momentR, moment.momentS)
	confidence := 0.0

	if theoreticalOK {
		confidence = MomentConfidence(empirical, theoretical)
	}

	moment.artifact.Poke(datura.Map[float64]{
		"value":       confidence,
		"empirical":   empirical,
		"theoretical": theoretical,
		"confidence":  confidence,
	}, "output")

	return moment.artifact.Read(p)
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
		artifact:  datura.Acquire("hawkes-fit", datura.APPJSON).RetainStageAttributes(),
		horizon:   horizonUnixNano,
		prior:     prior,
		estimator: NewBivariateEstimator(prior),
	}
}

func (fit *Fit) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](fit.artifact, "output") == nil

	fit.artifact.Clear("sample")
	fit.artifact.Clear("batch")

	n, err := fit.artifact.Write(p)

	if bootstrap {
		fit.artifact.Clear("output")
	}

	return n, err
}

func (fit *Fit) Read(p []byte) (int, error) {
	xTimes, yTimes, ok := fitTimes(fit.artifact)

	if !ok {
		fit.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return fit.artifact.Read(p)
	}

	stream := NewArrivalStream(fitTimesToTime(xTimes), fitTimesToTime(yTimes))
	horizon := fitHorizon(fit)
	fitted := fit.estimator.Fit(stream, horizon)

	if !fitted.Valid() {
		fit.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return fit.artifact.Read(p)
	}

	asymmetry := fitted.Asymmetry(false)
	ratio := 0.0

	if asymmetry > 0 && fitted.MuX > 0 {
		ratio = fitted.IntensityX / fitted.MuX
	}

	if asymmetry <= 0 && fitted.MuY > 0 {
		ratio = fitted.IntensityY / fitted.MuY
	}

	fit.artifact.Poke(datura.Map[float64]{
		"value":           ratio,
		"excitationRatio": ratio,
		"spectralRadius":  fitted.SpectralRadius,
		"asymmetry":       asymmetry,
	}, "output")

	return fit.artifact.Read(p)
}

func (fit *Fit) Close() error {
	return nil
}

func momentSamples(artifact *datura.Artifact) (xValues, yValues, weights []float64, ok bool) {
	batch := datura.Peek[[]float64](artifact, "batch")

	if len(batch) == 0 {
		batch = fitFloatBatch(artifact)
	}

	if len(batch) < 4 || len(batch)%2 != 0 {
		return nil, nil, nil, false
	}

	half := len(batch) / 2
	xValues = batch[:half]
	yValues = batch[half:]
	weights = datura.Peek[[]float64](artifact, "config", "weights")

	if len(weights) == 0 {
		weights = nil
	}

	ok = len(xValues) == len(yValues) && len(xValues) >= 2

	if len(weights) != 0 && len(weights) != len(xValues) {
		ok = false
	}

	return xValues, yValues, weights, ok
}

func fitTimes(artifact *datura.Artifact) (xTimes, yTimes []float64, ok bool) {
	xCount := int(datura.Peek[float64](artifact, "config", "xCount"))
	yCount := int(datura.Peek[float64](artifact, "config", "yCount"))
	batch := datura.Peek[[]float64](artifact, "batch")

	if len(batch) == 0 {
		batch = fitFloatBatch(artifact)
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
	payload, ok := artifact.PayloadQuiet()

	if !ok || len(payload) == 0 || len(payload)%8 != 0 {
		return nil
	}

	samples := make([]float64, len(payload)/8)

	for index := range samples {
		offset := index * 8
		value := math.Float64frombits(
			uint64(payload[offset])<<56 |
				uint64(payload[offset+1])<<48 |
				uint64(payload[offset+2])<<40 |
				uint64(payload[offset+3])<<32 |
				uint64(payload[offset+4])<<24 |
				uint64(payload[offset+5])<<16 |
				uint64(payload[offset+6])<<8 |
				uint64(payload[offset+7]),
		)

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
