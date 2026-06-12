package core

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestPipeline_Observe(testingTB *testing.T) {
	Convey("Given a pipeline with a nil stage", testingTB, func() {
		pipeline := &Pipeline{
			stages: []Number{nil},
			work:   make([]Float64, 0, 8),
		}

		Convey("When observing", func() {
			_, err := pipeline.Observe(Float64(1))

			Convey("It should return ErrNilNumber", func() {
				So(err, ShouldEqual, ErrNilNumber)
			})
		})
	})

	Convey("Given a pipeline with a failing stage", testingTB, func() {
		pipeline := &Pipeline{
			stages: []Number{errStage{err: errStageFailed}},
			work:   make([]Float64, 0, 8),
		}

		Convey("When observing", func() {
			_, err := pipeline.Observe(Float64(1))

			Convey("It should return the stage error", func() {
				So(err, ShouldEqual, errStageFailed)
			})
		})
	})

	Convey("Given a pipeline with zero-capacity work", testingTB, func() {
		pipeline := &Pipeline{
			stages: []Number{fastStage{}},
		}

		Convey("When observing", func() {
			value, err := pipeline.Observe(Float64(7))

			Convey("It should grow the work buffer", func() {
				So(err, ShouldBeNil)
				So(value, ShouldEqual, 7)
				So(cap(pipeline.work), ShouldBeGreaterThanOrEqualTo, 1)
			})
		})
	})

	Convey("Given a pipeline with a working stage", testingTB, func() {
		pipeline := NewPipeline([]Number{echoStage{}})

		Convey("When observing", func() {
			value, err := pipeline.Observe(Float64(3))

			Convey("It should propagate the raw sample", func() {
				So(err, ShouldBeNil)
				So(value, ShouldEqual, 3)
			})
		})
	})

	Convey("Given a pipeline with a fast stage", testingTB, func() {
		pipeline := NewPipeline([]Number{fastStage{}})

		Convey("When observing", func() {
			value, err := pipeline.Observe(Float64(6))

			Convey("It should use Apply without error", func() {
				So(err, ShouldBeNil)
				So(value, ShouldEqual, 6)
			})
		})
	})

	Convey("Given a pipeline with two fast stages", testingTB, func() {
		pipeline := NewPipeline([]Number{fastStage{}, fastStage{}})

		Convey("When observing", func() {
			value, err := pipeline.Observe(Float64(2))

			Convey("It should run both stages", func() {
				So(err, ShouldBeNil)
				So(value, ShouldEqual, 2)
			})
		})
	})

	Convey("Given a pipeline with a slow stage", testingTB, func() {
		pipeline := NewPipeline([]Number{echoStage{}})

		Convey("When observing", func() {
			value, err := pipeline.Observe(Float64(4))

			Convey("It should use buildStageInputs", func() {
				So(err, ShouldBeNil)
				So(value, ShouldEqual, 4)
			})
		})
	})
}

func TestNewPipeline(testingTB *testing.T) {
	Convey("Given stage numbers", testingTB, func() {
		pipeline := NewPipeline([]Number{Float64(1)})

		Convey("It should expand stages through the registry", func() {
			So(pipeline, ShouldNotBeNil)
			So(len(pipeline.stages), ShouldEqual, 1)
		})
	})
}

func TestNewPipelineWithRegistry(testingTB *testing.T) {
	Convey("Given a dedicated registry", testingTB, func() {
		registry := NewBoundaryRegistry()
		pipeline := NewPipelineWithRegistry(registry, []Number{echoStage{}})

		Convey("It should bind the registry", func() {
			So(pipeline.registry, ShouldEqual, registry)
		})
	})
}

func BenchmarkPipeline_Observe(testingTB *testing.B) {
	pipeline := NewPipeline([]Number{fastStage{}})

	for testingTB.Loop() {
		_, _ = pipeline.Observe(Float64(1))
	}
}
