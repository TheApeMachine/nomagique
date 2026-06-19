package causal

import (
	"strconv"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
)

func TestZip_Read(testingTB *testing.T) {
	cases := []struct {
		name      string
		streams   [][]float64
		wantRows  float64
		wantTable bool
	}{
		{name: "empty", streams: nil, wantRows: 0},
		{name: "empty row", streams: [][]float64{{}}, wantRows: 0},
		{name: "mismatched", streams: [][]float64{{1, 2}, {1}}, wantRows: 0},
		{name: "valid", streams: [][]float64{{1, 2}, {3, 4}}, wantRows: 2, wantTable: true},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given zip with "+testCase.name, testingTB, func() {
			zipStage := NewZip()
			artifact := datura.Acquire("test", datura.APPJSON)

			if len(testCase.streams) > 0 {
				artifact.Poke(float64(len(testCase.streams)), "streams", "nodeCount")

				for nodeIndex, stream := range testCase.streams {
					artifact.Poke(stream, "streams", strconv.Itoa(nodeIndex))
				}
			}

			err := transport.NewFlipFlop(artifact, zipStage)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, testCase.wantRows)

			if testCase.wantTable {
				So(datura.Peek[float64](artifact, "table", "rowCount"), ShouldEqual, testCase.wantRows)
				So(datura.Peek[float64](artifact, "table", "nodeCount"), ShouldEqual, 2)
				So(datura.Peek[[]float64](artifact, "table", "rows"), ShouldResemble, []float64{1, 3, 2, 4})
			}
		})
	}
}

func TestNodeRingZip_Read(testingTB *testing.T) {
	Convey("Given aligned node observations through NodeRing and Zip", testingTB, func() {
		pipeline := nomagique.Number(NewNodeRing(), NewZip())
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(4, "config", "nodeCount").
			Poke(8, "config", "capacity")

		for index := range 16 {
			artifact.Poke([]float64{
				float64(index) * 0.1,
				float64(index) * 0.2,
				float64(index) * 0.5,
				float64(index) * 0.05,
			}, "batch")
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)
		}

		Convey("It should retain bounded aligned table rows", func() {
			So(datura.Peek[float64](artifact, "table", "rowCount"), ShouldEqual, 8)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 8)
		})
	})

	Convey("Given partial node inputs", testingTB, func() {
		nodeRing := NewNodeRing()
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(4, "config", "nodeCount").
			Poke(8, "config", "capacity").
			Poke([]float64{1}, "batch")
		err := transport.NewFlipFlop(artifact, nodeRing)

		Convey("It should ignore misaligned rows", func() {
			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)
			So(datura.Peek[float64](artifact, "streams", "nodeCount"), ShouldEqual, 0)
		})
	})
}

func BenchmarkZip_Read(testingTB *testing.B) {
	zipStage := NewZip()
	artifact := datura.Acquire("test", datura.APPJSON).
		Poke(float64(4), "streams", "nodeCount").
		Poke([]float64{0.1, 0.2, 0.3, 0.4}, "streams", "0").
		Poke([]float64{0.5, 0.6, 0.7, 0.8}, "streams", "1").
		Poke([]float64{1.0, 1.1, 1.2, 1.3}, "streams", "2").
		Poke([]float64{2.0, 2.1, 2.2, 2.3}, "streams", "3")

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, zipStage)
	}
}
