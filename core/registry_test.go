package core

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBoundaryRegistry_RegisterStages(testingTB *testing.T) {
	Convey("Given a registry", testingTB, func() {
		registry := NewBoundaryRegistry()
		stages := []Number{echoStage{}}

		Convey("When registering stages only", func() {
			registry.RegisterStages(Float64(11), stages)
			resolved, registered := registry.StagesFor(Float64(11))

			Convey("It should resolve the stages", func() {
				So(registered, ShouldBeTrue)
				So(len(resolved), ShouldEqual, 1)
			})
		})
	})
}

func TestBoundaryRegistry_Register(testingTB *testing.T) {
	Convey("Given a registry", testingTB, func() {
		registry := NewBoundaryRegistry()
		pipeline := &Pipeline{stages: []Number{Float64(2)}}

		Convey("When registering a boundary", func() {
			registry.Register(Float64(3), pipeline)
			expanded := registry.ExpandNumbers([]Number{Float64(3)})

			Convey("It should flatten to the stored stages", func() {
				So(len(expanded), ShouldEqual, 1)
				So(expanded[0], ShouldEqual, Float64(2))
			})
		})
	})
}

func TestBoundaryRegistry_StagesFor(testingTB *testing.T) {
	Convey("Given an empty stack for a token", testingTB, func() {
		registry := NewBoundaryRegistry()
		registry.stacks[Float64(5)] = []*Pipeline{}

		Convey("When resolving stages", func() {
			stages, registered := registry.StagesFor(Float64(5))

			Convey("It should not be registered", func() {
				So(registered, ShouldBeFalse)
				So(stages, ShouldBeNil)
			})
		})
	})
}

func TestBoundaryRegistry_ExpandNumbers(testingTB *testing.T) {
	Convey("Given an unregistered boundary token", testingTB, func() {
		registry := NewBoundaryRegistry()
		token := Float64(7)

		Convey("When expanding", func() {
			expanded := registry.ExpandNumbers([]Number{token})

			Convey("It should pass through unchanged", func() {
				So(expanded[0], ShouldEqual, token)
			})
		})
	})

	Convey("Given a non-boundary number", testingTB, func() {
		registry := NewBoundaryRegistry()

		Convey("When expanding", func() {
			expanded := registry.ExpandNumbers([]Number{notFloatNumber{}})

			Convey("It should pass through unchanged", func() {
				So(len(expanded), ShouldEqual, 1)
			})
		})
	})
}

func BenchmarkBoundaryRegistry_ExpandNumbers(testingTB *testing.B) {
	registry := NewBoundaryRegistry()
	registry.Register(Float64(1), &Pipeline{stages: []Number{echoStage{}}})
	numbers := []Number{Float64(1)}

	for testingTB.Loop() {
		_ = registry.ExpandNumbers(numbers)
	}
}
