package core

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestAcquirePipeline(testingTB *testing.T) {
	Convey("Given pooled pipelines", testingTB, func() {
		first := AcquirePipeline([]Number{echoStage{}})
		ReleasePipeline(first)
		second := AcquirePipeline([]Number{echoStage{}})

		Convey("It should reuse the pooled instance", func() {
			So(second, ShouldEqual, first)
		})

		ReleasePipeline(second)
	})
}

func TestPipeline_Bind(testingTB *testing.T) {
	Convey("Given a pooled pipeline", testingTB, func() {
		pipeline := AcquirePipeline([]Number{echoStage{}})

		Convey("When rebound to new stages", func() {
			pipeline.Bind([]Number{Float64(2)})

			Convey("It should replace stages", func() {
				So(len(pipeline.stages), ShouldEqual, 1)
			})
		})

		ReleasePipeline(pipeline)
	})
}
