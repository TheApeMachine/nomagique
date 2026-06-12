package core

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFloat64_Observe(testingTB *testing.T) {
	Convey("Given an unregistered boundary token", testingTB, func() {
		boundary := Float64(4)

		Convey("When observing with a float sample", func() {
			value := boundary.Observe(Float64(4))

			Convey("It should return the sample", func() {
				So(value, ShouldEqual, 4)
			})
		})
	})

	Convey("Given a registered boundary token", testingTB, func() {
		token := Float64(99)
		pipeline := NewPipeline([]Number{echoStage{}})
		DefaultRegistry.Register(token, pipeline)

		Convey("When observing", func() {
			value := token.Observe(Float64(10))

			Convey("It should run the nested pipeline", func() {
				So(value, ShouldEqual, 10)
			})
		})
	})

	Convey("Given empty inputs", testingTB, func() {
		boundary := Float64(1)

		Convey("When observing", func() {
			value := boundary.Observe()

			Convey("It should return zero", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})

	Convey("Given a non-float input", testingTB, func() {
		boundary := Float64(1)

		Convey("When observing", func() {
			value := boundary.Observe(notFloatNumber{})

			Convey("It should return zero", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})
}

func TestFloat64_Reset(testingTB *testing.T) {
	Convey("Given a boundary float", testingTB, func() {
		boundary := Float64(0)

		Convey("When reset", func() {
			err := boundary.Reset()

			Convey("It should succeed", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}

func BenchmarkFloat64_Observe(testingTB *testing.B) {
	boundary := Float64(2)

	for testingTB.Loop() {
		_ = boundary.Observe(Float64(2))
	}
}
