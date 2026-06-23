package hawkes

import (
	"encoding/binary"
	"math"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
	"gonum.org/v1/gonum/stat"
)

/*
Moment validates bivariate exponential-kernel parameters through empirical moments.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Moment struct {
	artifact *datura.Artifact
}

/*
NewMoment creates a Hawkes moment-confidence stage wired from config attributes.
momentR and momentS on the artifact select the mixed moment used for fit diagnostics.
*/
func NewMoment(artifact *datura.Artifact) *Moment {
	if artifact == nil {
		artifact = datura.Acquire("hawkes-moment", datura.APPJSON)
	}

	artifact.Inspect("hawkes", "moment", "NewMoment()")

	return &Moment{
		artifact: artifact,
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

	params := bivariateParamsFromArtifact(moment.artifact)

	if !statistic.KeyPresent(moment.artifact, "config", "momentR") ||
		!statistic.KeyPresent(moment.artifact, "config", "momentS") {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes-moment: config momentR and momentS required",
			nil,
		))
	}

	momentR := datura.Peek[float64](moment.artifact, "config", "momentR")
	momentS := datura.Peek[float64](moment.artifact, "config", "momentS")

	empirical := stat.BivariateMoment(momentR, momentS, xValues, yValues, weights)
	theoretical, theoreticalOK := TheoreticalCentralMoment(params, momentR, momentS)

	if !theoreticalOK {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes-moment: theoretical moment unavailable for parameters",
			nil,
		))
	}

	confidence, confidenceOK := MomentConfidence(empirical, theoretical)

	if !confidenceOK {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes-moment: confidence could not be derived",
			nil,
		))
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
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Fit struct {
	artifact *datura.Artifact
}

/*
NewFit creates a timestamp-stream Hawkes fit stage wired from config attributes.
horizonUnixNano on the artifact is the observation horizon in Unix nanoseconds.
*/
func NewFit(artifact *datura.Artifact) *Fit {
	if artifact == nil {
		artifact = datura.Acquire("hawkes-fit", datura.APPJSON)
	}

	artifact.Inspect("hawkes", "fit", "NewFit()")

	return &Fit{
		artifact: artifact,
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
	horizon := fitHorizon(fit.artifact)
	prior := bivariateFitFromArtifact(fit.artifact)
	fitted := NewBivariateEstimator(prior).Fit(stream, horizon)

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
	features := streamFeatures(wire)

	if len(features) == 0 {
		return nil, nil, nil, false
	}

	xCount, yCount := streamCounts(wire, config, features)

	if xCount > 0 && yCount > 0 && len(features) >= xCount+yCount {
		xValues = append([]float64(nil), features[:xCount]...)
		yValues = append([]float64(nil), features[xCount:xCount+yCount]...)
	} else if len(features) < 4 || len(features)%2 != 0 {
		return nil, nil, nil, false
	} else {
		half := len(features) / 2
		xValues = append([]float64(nil), features[:half]...)
		yValues = append([]float64(nil), features[half:]...)
	}

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
	features := streamFeatures(wire)

	if len(features) == 0 {
		return nil, nil, false
	}

	xCount, yCount := streamCounts(wire, config, features)

	if xCount <= 0 || yCount <= 0 || len(features) < xCount+yCount {
		return nil, nil, false
	}

	xTimes = features[:xCount]
	yTimes = features[xCount : xCount+yCount]

	for _, sample := range append(xTimes, yTimes...) {
		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return nil, nil, false
		}
	}

	return xTimes, yTimes, len(xTimes)+len(yTimes) >= 2
}

func streamFeatures(wire *datura.Artifact) []float64 {
	features := equation.Features(wire)

	if len(features) > 0 {
		return features
	}

	features = fitFloatBatch(wire)

	if len(features) > 0 {
		return features
	}

	return datura.Peek[[]float64](wire, "batch")
}

func streamCounts(wire, config *datura.Artifact, features []float64) (int, int) {
	xCount := int(countField(wire, "xCount"))
	yCount := int(countField(wire, "yCount"))

	if xCount <= 0 {
		xCount = int(datura.Peek[float64](wire, "config", "xCount"))
	}

	if yCount <= 0 {
		yCount = int(datura.Peek[float64](wire, "config", "yCount"))
	}

	if xCount <= 0 {
		xCount = int(datura.Peek[float64](config, "config", "xCount"))
	}

	if yCount <= 0 {
		yCount = int(datura.Peek[float64](config, "config", "yCount"))
	}

	if xCount <= 0 && yCount <= 0 && len(features) >= 4 && len(features)%2 == 0 {
		xCount = len(features) / 2
		yCount = len(features) / 2
	}

	return xCount, yCount
}

func countField(wire *datura.Artifact, key string) float64 {
	value, err := statistic.FeatureColumn(wire, key)

	if err != nil {
		return 0
	}

	return value
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

func fitHorizon(artifact *datura.Artifact) time.Time {
	horizonNano := int64(datura.Peek[float64](artifact, "config", "horizonUnixNano"))

	return time.Unix(0, horizonNano)
}

func bivariateParamsFromArtifact(artifact *datura.Artifact) BivariateParams {
	return BivariateParams{
		MuX:     datura.Peek[float64](artifact, "config", "muX"),
		MuY:     datura.Peek[float64](artifact, "config", "muY"),
		AlphaXX: datura.Peek[float64](artifact, "config", "alphaXX"),
		AlphaXY: datura.Peek[float64](artifact, "config", "alphaXY"),
		AlphaYX: datura.Peek[float64](artifact, "config", "alphaYX"),
		AlphaYY: datura.Peek[float64](artifact, "config", "alphaYY"),
		Beta:    datura.Peek[float64](artifact, "config", "beta"),
	}
}

func bivariateFitFromArtifact(artifact *datura.Artifact) BivariateFit {
	return BivariateFit{
		MuX:            datura.Peek[float64](artifact, "config", "muX"),
		MuY:            datura.Peek[float64](artifact, "config", "muY"),
		AlphaXX:        datura.Peek[float64](artifact, "config", "alphaXX"),
		AlphaXY:        datura.Peek[float64](artifact, "config", "alphaXY"),
		AlphaYX:        datura.Peek[float64](artifact, "config", "alphaYX"),
		AlphaYY:        datura.Peek[float64](artifact, "config", "alphaYY"),
		Beta:           datura.Peek[float64](artifact, "config", "beta"),
		IntensityX:     datura.Peek[float64](artifact, "config", "intensityX"),
		IntensityY:     datura.Peek[float64](artifact, "config", "intensityY"),
		SpectralRadius: datura.Peek[float64](artifact, "config", "spectralRadius"),
	}
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
