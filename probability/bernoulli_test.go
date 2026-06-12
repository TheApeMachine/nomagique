package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestBernoulli(testingTB *testing.T) {
	Convey("Given Bernoulli constructor", testingTB, func() {
		posterior := Bernoulli()

		Convey("It should return a usable dynamic", func() {
			So(posterior, ShouldNotBeNil)
		})
	})
}

func TestPosterior_Observe(testingTB *testing.T) {
	Convey("Given a unit-interval outcome", testingTB, func() {
		posterior := Bernoulli()

		Convey("When observing", func() {
			value := posterior.Observe(core.Float64(1))

			Convey("It should return a posterior mean", func() {
				So(float64(value), ShouldBeGreaterThan, 0.5)
			})
		})
	})

	Convey("Given a predicted and actual pair", testingTB, func() {
		posterior := Bernoulli()
		posterior.Observe(core.Float64(10), core.Float64(10))
		value := posterior.Observe(core.Float64(10), core.Float64(15))

		Convey("It should raise hit probability", func() {
			So(float64(value), ShouldBeGreaterThan, 0.5)
		})
	})

	Convey("Given an invalid outcome", testingTB, func() {
		posterior := Bernoulli()

		Convey("When observing", func() {
			value := posterior.Observe(core.Float64(2))

			Convey("It should return ErrInvalidOutcome", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

func BenchmarkBernoulli_Observe(testingTB *testing.B) {
	posterior := Bernoulli()
	posterior.Observe(core.Float64(1))

	for testingTB.Loop() {
		posterior.Observe(core.Float64(10), core.Float64(11))
	}
}
