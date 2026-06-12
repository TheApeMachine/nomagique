package algorithm

import (
	"time"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/kernel/hawkes"
)

/*
HawkesFit estimates bivariate Hawkes parameters from timestamp arrival streams.
*/
type HawkesFit struct {
	xTimes    core.Numbers
	yTimes    core.Numbers
	horizon   time.Time
	prior     hawkes.BivariateFit
	estimator *hawkes.BivariateEstimator
	fit       hawkes.BivariateFit
}

/*
NewHawkesFit creates a timestamp-stream Hawkes fit dynamic.
horizonUnixNano is the observation horizon in Unix nanoseconds.
*/
func NewHawkesFit(
	xTimes, yTimes core.Numbers,
	horizonUnixNano float64,
	prior hawkes.BivariateFit,
) *HawkesFit {
	return &HawkesFit{
		xTimes:    xTimes,
		yTimes:    yTimes,
		horizon:   time.Unix(0, int64(horizonUnixNano)),
		prior:     prior,
		estimator: hawkes.NewBivariateEstimator(prior),
	}
}

/*
Observe fits the stream and returns dominant-side excitation ratio λ/μ.
*/
func (hawkesFit *HawkesFit) Observe(_ ...core.Number) core.Float64 {
	stream, ok := hawkesFit.arrivalStream()

	if !ok {
		return 0
	}

	hawkesFit.fit = hawkesFit.estimator.Fit(stream, hawkesFit.horizon)

	if !hawkesFit.fit.Valid() {
		return 0
	}

	asymmetry := hawkesFit.fit.Asymmetry(false)

	if asymmetry > 0 && hawkesFit.fit.MuX > 0 {
		return core.Float64(hawkesFit.fit.IntensityX / hawkesFit.fit.MuX)
	}

	if hawkesFit.fit.MuY > 0 {
		return core.Float64(hawkesFit.fit.IntensityY / hawkesFit.fit.MuY)
	}

	return 0
}

/*
Fit returns the last MLE fit from Observe.
*/
func (hawkesFit *HawkesFit) Fit() (hawkes.BivariateFit, bool) {
	return hawkesFit.fit, hawkesFit.fit.Valid()
}

/*
Asymmetry returns normalized intensity excess on the requested side.
*/
func (hawkesFit *HawkesFit) Asymmetry(preferY bool) core.Float64 {
	return core.Float64(hawkesFit.fit.Asymmetry(preferY))
}

/*
SpectralRadius returns the fitted branching spectral radius.
*/
func (hawkesFit *HawkesFit) SpectralRadius() core.Float64 {
	return core.Float64(hawkesFit.fit.SpectralRadius)
}

/*
Category returns the classified fit regime and confidence.
*/
func (hawkesFit *HawkesFit) Category(preferY bool) (hawkes.FitCategory, core.Float64) {
	category, confidence := hawkes.ClassifyFit(
		hawkesFit.fit, hawkesFit.fit.Asymmetry(preferY), preferY,
	)

	return category, core.Float64(confidence)
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
	xTimes := samplesToTimes(nomagique.Samples(hawkesFit.xTimes))
	yTimes := samplesToTimes(nomagique.Samples(hawkesFit.yTimes))

	if len(xTimes)+len(yTimes) < 2 {
		return hawkes.ArrivalStream{}, false
	}

	return hawkes.NewArrivalStream(xTimes, yTimes), true
}

func samplesToTimes(samples []float64) []time.Time {
	times := make([]time.Time, len(samples))

	for index, sample := range samples {
		times[index] = time.Unix(0, int64(sample))
	}

	return times
}
