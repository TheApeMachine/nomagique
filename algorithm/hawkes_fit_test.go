package algorithm

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/hawkes"
)

func TestHawkesFit_Observe(testingTB *testing.T) {
	Convey("Given timestamp arrival streams with enough events", testingTB, func() {
		start := time.Now()
		xTimes := make([]float64, 32)
		yTimes := make([]float64, 32)

		for index := range xTimes {
			xTimes[index] = float64(
				start.Add(time.Duration(index) * 100 * time.Millisecond).UnixNano(),
			)
			yTimes[index] = float64(
				start.Add(time.Duration(index)*100*time.Millisecond + 50*time.Millisecond).UnixNano(),
			)
		}

		horizon := float64(start.Add(4 * time.Second).UnixNano())
		fitProcess := NewHawkesFit(
			nomagique.Numbers(xTimes...),
			nomagique.Numbers(yTimes...),
			horizon,
			hawkes.BivariateFit{},
		)

		excitation := fitProcess.Observe()
		fit, ok := fitProcess.Fit()

		Convey("It should fit and return a positive excitation ratio", func() {
			So(ok, ShouldBeTrue)
			So(fit.MuX, ShouldBeGreaterThan, 0)
			So(float64(excitation), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkHawkesFit_Observe(testingTB *testing.B) {
	start := time.Now()
	xTimes := make([]float64, 32)
	yTimes := make([]float64, 32)

	for index := range xTimes {
		xTimes[index] = float64(start.Add(time.Duration(index) * 100 * time.Millisecond).UnixNano())
		yTimes[index] = float64(start.Add(time.Duration(index)*100*time.Millisecond + 50*time.Millisecond).UnixNano())
	}

	horizon := float64(start.Add(4 * time.Second).UnixNano())
	fitProcess := NewHawkesFit(
		nomagique.Numbers(xTimes...),
		nomagique.Numbers(yTimes...),
		horizon,
		hawkes.BivariateFit{},
	)

	for testingTB.Loop() {
		_ = fitProcess.Observe()
	}
}
