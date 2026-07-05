package hawkes

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFitMeasure(testingTB *testing.T) {
	Convey("Given arrival timestamps with enough events", testingTB, func() {
		start := time.Now()
		xTimes := make([]time.Time, 32)
		yTimes := make([]time.Time, 32)

		for index := range xTimes {
			xTimes[index] = start.Add(time.Duration(index) * 100 * time.Millisecond)
			yTimes[index] = start.Add(time.Duration(index)*100*time.Millisecond + 50*time.Millisecond)
		}

		fitStage, err := NewFit(FitConfig{Horizon: start.Add(4 * time.Second)})
		So(err, ShouldBeNil)

		output, err := fitStage.Measure(FitInput{
			XTimes: xTimes,
			YTimes: yTimes,
		})

		Convey("It should fit and return positive excitation evidence", func() {
			So(err, ShouldBeNil)
			So(output.Fit.Valid(), ShouldBeTrue)
			So(output.SpectralRadius, ShouldBeGreaterThanOrEqualTo, 0)
			So(output.Value, ShouldBeGreaterThan, 0)
		})
	})
}

func TestMomentMeasureRequiresAlignedSamples(testingTB *testing.T) {
	Convey("Given insufficient moment samples", testingTB, func() {
		moment, err := NewMoment(MomentConfig{
			Params:  BivariateParams{MuX: 1, MuY: 1, Beta: 1},
			MomentR: 1,
			MomentS: 1,
		})
		So(err, ShouldBeNil)

		_, err = moment.Measure(MomentInput{})

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestFitReadRequiresAlignedTimestamps(testingTB *testing.T) {
	Convey("Given insufficient fit timestamps", testingTB, func() {
		fitStage, err := NewFit(FitConfig{Horizon: time.Now()})
		So(err, ShouldBeNil)

		_, err = fitStage.Measure(FitInput{})

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}
