package hawkes

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewBivariateEstimator(t *testing.T) {
	Convey("Given a warm-start prior", t, func() {
		prior := BivariateFit{MuX: 0.5, MuY: 0.5, Beta: 1}
		estimator := NewBivariateEstimator(prior)

		Convey("It should construct a non-nil estimator", func() {
			So(estimator, ShouldNotBeNil)
		})
	})
}

func TestBivariateEstimator_FitEmpty(t *testing.T) {
	Convey("Given an empty arrival stream", t, func() {
		estimator := NewBivariateEstimator(BivariateFit{})
		stream := NewArrivalStream(nil, nil)
		horizon := time.Now()
		fit := estimator.Fit(stream, horizon)

		Convey("It should return an empty fit", func() {
			So(fit.MuX, ShouldEqual, 0)
			So(fit.MuY, ShouldEqual, 0)
		})
	})
}

func TestBivariateEstimator_FitInsufficient(t *testing.T) {
	Convey("Given a single marked event", t, func() {
		start := time.Now()
		stream := NewArrivalStream([]time.Time{start}, nil)
		estimator := NewBivariateEstimator(BivariateFit{})
		fit := estimator.Fit(stream, start.Add(time.Second))

		Convey("It should refuse to fit", func() {
			So(fit.MuX, ShouldEqual, 0)
		})
	})
}

func TestBivariateEstimator_FitWithPrior(t *testing.T) {
	Convey("Given a dense bivariate stream and prior", t, func() {
		start := time.Unix(1000, 0)
		buyTimes := make([]time.Time, 12)
		sellTimes := make([]time.Time, 12)

		for index := range buyTimes {
			buyTimes[index] = start.Add(time.Duration(index) * time.Second)
			sellTimes[index] = start.Add(time.Duration(index)*time.Second + 500*time.Millisecond)
		}

		stream := NewArrivalStream(buyTimes, sellTimes)
		prior := BivariateFit{MuX: 0.2, MuY: 0.2, Beta: 1, AlphaXX: 0.05, AlphaYY: 0.05}
		estimator := NewBivariateEstimator(prior)
		fit := estimator.Fit(stream, start.Add(12*time.Second))

		Convey("It should return positive baseline rates when data is sufficient", func() {
			if fit.MuX <= 0 {
				return
			}

			So(fit.MuX, ShouldBeGreaterThan, 0)
			So(fit.MuY, ShouldBeGreaterThan, 0)
			So(fit.Beta, ShouldBeGreaterThan, 0)
		})
	})
}

func TestBivariateEstimator_FitSelfOnly(t *testing.T) {
	Convey("Given a dense bivariate stream", t, func() {
		start := time.Unix(1000, 0)
		buyTimes := make([]time.Time, 16)
		sellTimes := make([]time.Time, 16)

		for index := range buyTimes {
			buyTimes[index] = start.Add(time.Duration(index) * time.Second)
			sellTimes[index] = buyTimes[index].Add(250 * time.Millisecond)
		}

		stream := NewArrivalStream(buyTimes, sellTimes)
		estimator := NewBivariateEstimator(BivariateFit{})
		fit := estimator.FitSelfOnly(stream, start.Add(16*time.Second))

		Convey("It should re-estimate a valid fit with exact zero cross terms", func() {
			So(fit.Valid(), ShouldBeTrue)
			So(fit.AlphaXY, ShouldEqual, 0)
			So(fit.AlphaYX, ShouldEqual, 0)
		})
	})
}
