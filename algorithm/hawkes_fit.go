package algorithm

import (
	"time"

	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/hawkes"
)

/*
HawkesFit estimates bivariate Hawkes parameters from timestamp arrival streams.
*/
type HawkesFit[T ~float64] struct {
	xTimes        []float64
	yTimes        []float64
	horizon       time.Time
	prior         hawkes.BivariateFit
	estimator     *hawkes.BivariateEstimator
	fit           hawkes.BivariateFit
	spectralRadii []float64
	asymmetries   []float64
	output        core.Scalar[T]
}

/*
NewHawkesFit creates a timestamp-stream Hawkes fit dynamic.
horizonUnixNano is the observation horizon in Unix nanoseconds.
*/
func NewHawkesFit[T ~float64](
	xTimes, yTimes []float64,
	horizonUnixNano float64,
	prior hawkes.BivariateFit,
) *HawkesFit[T] {
	return &HawkesFit[T]{
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
func (hawkesFit *HawkesFit[T]) Observe(_ ...core.Number[T]) core.Scalar[T] {
	stream, ok := hawkesFit.arrivalStream()

	if !ok {
		return hawkesFit.output
	}

	hawkesFit.fit = hawkesFit.estimator.Fit(stream, hawkesFit.horizon)

	if !hawkesFit.fit.Valid() {
		return hawkesFit.output
	}

	hawkesFit.recordFitGates()

	asymmetry := hawkesFit.fit.Asymmetry(false)

	if asymmetry > 0 && hawkesFit.fit.MuX > 0 {
		hawkesFit.output = core.Scalar[T](T(hawkesFit.fit.IntensityX / hawkesFit.fit.MuX))

		return hawkesFit.output
	}

	if hawkesFit.fit.MuY > 0 {
		hawkesFit.output = core.Scalar[T](T(hawkesFit.fit.IntensityY / hawkesFit.fit.MuY))

		return hawkesFit.output
	}

	return hawkesFit.output
}

/*
Fit returns the last MLE fit from Observe.
*/
func (hawkesFit *HawkesFit[T]) Fit() (hawkes.BivariateFit, bool) {
	return hawkesFit.fit, hawkesFit.fit.Valid()
}

/*
Asymmetry returns normalized intensity excess on the requested side.
*/
func (hawkesFit *HawkesFit[T]) Asymmetry(preferY bool) core.Scalar[T] {
	return core.Scalar[T](T(hawkesFit.fit.Asymmetry(preferY)))
}

/*
SpectralRadius returns the fitted branching spectral radius.
*/
func (hawkesFit *HawkesFit[T]) SpectralRadius() core.Scalar[T] {
	return core.Scalar[T](T(hawkesFit.fit.SpectralRadius))
}

/*
Category returns the classified fit regime and confidence.
*/
func (hawkesFit *HawkesFit[T]) Category(preferY bool) (hawkes.FitCategory, core.Scalar[T]) {
	gates, gatesReady := hawkes.FitGatesFromHistory(hawkesFit.spectralRadii, hawkesFit.asymmetries)

	if !gatesReady {
		return hawkes.FitCategoryOrganic, core.Scalar[T](0)
	}

	category, confidence := hawkes.ClassifyFit(
		hawkesFit.fit, hawkesFit.fit.Asymmetry(preferY), preferY, gates,
	)

	return category, core.Scalar[T](T(confidence))
}

func (hawkesFit *HawkesFit[T]) recordFitGates() {
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

func appendRingFloat(values []float64, value float64, capacity int) []float64 {
	values = append(values, value)

	if len(values) <= capacity {
		return values
	}

	return values[len(values)-capacity:]
}

/*
Reset clears derived state.
*/
func (hawkesFit *HawkesFit[T]) Reset() error {
	hawkesFit.fit = hawkes.BivariateFit{}
	hawkesFit.output = core.Scalar[T](0)
	hawkesFit.estimator = hawkes.NewBivariateEstimator(hawkesFit.prior)

	return nil
}

func (hawkesFit *HawkesFit[T]) arrivalStream() (hawkes.ArrivalStream, bool) {
	xTimes := samplesToTimes(hawkesFit.xTimes)
	yTimes := samplesToTimes(hawkesFit.yTimes)

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
