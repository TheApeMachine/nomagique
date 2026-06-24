package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/equation"
)

func TestCausalStory_Read(testingTB *testing.T) {
	Convey("Given Pearl ladder outputs with endogenous uplift", testingTB, func() {
		stage := equation.NewCausalStory(equation.CausalStoryConfig())
		inbound := datura.Acquire("causal-story-in", datura.APPJSON)
		inbound.MergeOutput("association", 0.2)
		inbound.MergeOutput("intervention", 0.6)
		inbound.MergeOutput("uplift", 0.8)
		inbound.MergeOutput("contagion", 0.1)
		inbound.MergeOutput("condition", 0.2)
		inbound.MergeOutput("inverted", 0.0)

		err := transport.NewFlipFlop(inbound, stage)

		So(err, ShouldBeNil)

		Convey("It should emit alpha classification scores", func() {
			So(datura.Peek[float64](inbound, "output", "alphaScore"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](inbound, "output", "category"), ShouldEqual, 1)
		})
	})
}

func BenchmarkCausalStoryRead(b *testing.B) {
	stage := equation.NewCausalStory(equation.CausalStoryConfig())
	inbound := datura.Acquire("causal-story-bench", datura.APPJSON)
	inbound.MergeOutput("association", 0.2)
	inbound.MergeOutput("intervention", 0.6)
	inbound.MergeOutput("uplift", 0.8)
	inbound.MergeOutput("contagion", 0.1)
	inbound.MergeOutput("condition", 0.2)
	inbound.MergeOutput("inverted", 0.0)

	b.ReportAllocs()

	for b.Loop() {
		_ = transport.NewFlipFlop(inbound, stage)
	}
}
