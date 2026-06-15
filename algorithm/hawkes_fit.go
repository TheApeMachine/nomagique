package algorithm

import (
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/hawkes"
)

/*
HawkesFit estimates bivariate Hawkes parameters from timestamp arrival streams.
*/
type HawkesFit struct {
	artifact      *datura.Artifact
	xTimes        []float64
	yTimes        []float64
	horizon       time.Time
	prior         hawkes.BivariateFit
	estimator     *hawkes.BivariateEstimator
	fit           hawkes.BivariateFit
	spectralRadii []float64
	asymmetries   []float64
}

/*
NewHawkesFit creates a timestamp-stream Hawkes fit stage.
horizonUnixNano is the observation horizon in Unix nanoseconds.
*/
func NewHawkesFit(
	xTimes, yTimes []float64,
	horizonUnixNano float64,
	prior hawkes.BivariateFit,
) *HawkesFit {
	return &HawkesFit{
		artifact:  datura.Acquire("hawkes-fit", datura.Artifact_Type_json),
		xTimes:    xTimes,
		yTimes:    yTimes,
		horizon:   time.Unix(0, int64(horizonUnixNano)),
		prior:     prior,
		estimator: hawkes.NewBivariateEstimator(prior),
	}
}

func (hawkesFit *HawkesFit) Write(p []byte) (int, error) {
	return hawkesFit.artifact.Write(p)
}

func (hawkesFit *HawkesFit) Read(p []byte) (int, error) {
	rehydrateArtifact(&hawkesFit.artifact, "hawkes-fit", datura.Artifact_Type_json)

	stream, ok := hawkesFit.arrivalStream()

	if ok {
		hawkesFit.fit = hawkesFit.estimator.Fit(stream, hawkesFit.horizon)

		if hawkesFit.fit.Valid() {
			hawkesFit.recordFitGates()

			asymmetry := hawkesFit.fit.Asymmetry(false)

			if asymmetry > 0 && hawkesFit.fit.MuX > 0 {
				out := encodePayload(hawkesFit.fit.IntensityX / hawkesFit.fit.MuX)
				_ = hawkesFit.artifact.SetPayload(out)
			}

			if asymmetry <= 0 && hawkesFit.fit.MuY > 0 {
				out := encodePayload(hawkesFit.fit.IntensityY / hawkesFit.fit.MuY)
				_ = hawkesFit.artifact.SetPayload(out)
			}
		}
	}

	return hawkesFit.artifact.Read(p)
}

func (hawkesFit *HawkesFit) Close() error {
	return nil
}

/*
Fit returns the last MLE fit from Read.
*/
func (hawkesFit *HawkesFit) Fit() (hawkes.BivariateFit, bool) {
	return hawkesFit.fit, hawkesFit.fit.Valid()
}

/*
Asymmetry returns normalized intensity excess on the requested side.
*/
func (hawkesFit *HawkesFit) Asymmetry(preferY bool) float64 {
	return hawkesFit.fit.Asymmetry(preferY)
}

/*
SpectralRadius returns the fitted branching spectral radius.
*/
func (hawkesFit *HawkesFit) SpectralRadius() float64 {
	return hawkesFit.fit.SpectralRadius
}

/*
Category returns the classified fit regime and confidence.
*/
func (hawkesFit *HawkesFit) Category(preferY bool) (hawkes.FitCategory, float64) {
	gates, gatesReady := hawkes.FitGatesFromHistory(hawkesFit.spectralRadii, hawkesFit.asymmetries)

	if !gatesReady {
		return hawkes.FitCategoryOrganic, 0
	}

	category, confidence := hawkes.ClassifyFit(
		hawkesFit.fit, hawkesFit.fit.Asymmetry(preferY), preferY, gates,
	)

	return category, confidence
}

func (hawkesFit *HawkesFit) recordFitGates() {
	if hawkesFit.fit.SpectralRadius <= 0 {
		return
	}

	hawkesFit.spectralRadii = appendRingFloat(
		hawkesFit.spectralRadii,
		hawkesFit.fit.SpectralRadius,
		64,
	)
	hawkesFit.asymmetries = appendRingFloat(
		hawkesFit.asymmetries,
		hawkesFit.fit.Asymmetry(false),
		64,
	)
}

/*
Reset clears derived state.
*/
func (hawkesFit *HawkesFit) Reset() error {
	hawkesFit.fit = hawkes.BivariateFit{}
	hawkesFit.estimator = hawkes.NewBivariateEstimator(hawkesFit.prior)

	return nil
}

func (hawkesFit *HawkesFit) arrivalStream() (hawkes.ArrivalStream, bool) {
	xTimes := samplesToTimes(hawkesFit.xTimes)
	yTimes := samplesToTimes(hawkesFit.yTimes)

	if len(xTimes)+len(yTimes) < 2 {
		return hawkes.ArrivalStream{}, false
	}

	return hawkes.NewArrivalStream(xTimes, yTimes), true
}
