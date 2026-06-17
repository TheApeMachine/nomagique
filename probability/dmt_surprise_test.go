package probability

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/dmt"
)

func TestDMTSurprise_Read(testingTB *testing.T) {
	Convey("Given a classifier category through DMTSurprise", testingTB, func() {
		tree, treeErr := dmt.NewTree("")
		So(treeErr, ShouldBeNil)

		defer tree.Close()

		stage := NewDMTSurprise(tree, 4)
		inbound := datura.Acquire("dmt-transition-test", datura.Artifact_Type_json)
		pokeInt(inbound, "classifier.category", 2)
		buf, marshalErr := inbound.Message().Marshal()
		So(marshalErr, ShouldBeNil)
		_, writeErr := stage.Write(buf)
		So(writeErr, ShouldBeNil)

		got := readScalar(stage)

		Convey("It should return finite surprisal", func() {
			So(math.IsNaN(got), ShouldBeFalse)
		})
	})
}

func TestDMTSurprise_Contrastive(testingTB *testing.T) {
	Convey("Given repeated category observations", testingTB, func() {
		tree, treeErr := dmt.NewTree("")
		So(treeErr, ShouldBeNil)

		defer tree.Close()

		stage := NewDMTSurprise(tree, 4)

		for range 16 {
			inbound := datura.Acquire("dmt-transition-test", datura.Artifact_Type_json)
			pokeInt(inbound, "classifier.category", 2)
			buf, _ := inbound.Message().Marshal()
			_, _ = stage.Write(buf)
			_ = readStageOutput(stage)
		}

		inbound := datura.Acquire("dmt-transition-test", datura.Artifact_Type_json)
		pokeInt(inbound, "classifier.category", 3)
		buf, _ := inbound.Message().Marshal()
		_, _ = stage.Write(buf)
		got := readStageOutput(stage)

		Convey("It should emit positive surprisal on category change", func() {
			So(got, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkDMTSurprise_Read(testingTB *testing.B) {
	tree, _ := dmt.NewTree("")

	defer tree.Close()

	stage := NewDMTSurprise(tree, 4)
	inbound := datura.Acquire("dmt-transition-bench", datura.Artifact_Type_json)
	pokeInt(inbound, "classifier.category", 2)
	buf, _ := inbound.Message().Marshal()

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = stage.Write(buf)
		_ = readScalar(stage)
	}
}
