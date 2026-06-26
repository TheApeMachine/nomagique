package causal

import (
	"strconv"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func zipInbound(streams [][]float64) *datura.Artifact {
	artifact := datura.Acquire("zip-inbound", datura.APPJSON)

	if len(streams) == 0 {
		return artifact
	}

	artifact.Poke(float64(len(streams)), "streams", "nodeCount")

	for nodeIndex, stream := range streams {
		artifact.Poke(stream, "streams", strconv.Itoa(nodeIndex))
	}

	return artifact
}

func TestZip_Read(testingTB *testing.T) {
	cases := []struct {
		name      string
		streams   [][]float64
		wantErr   bool
		wantRows  float64
		wantTable bool
	}{
		{name: "empty", streams: nil, wantErr: true},
		{name: "empty row", streams: [][]float64{{}}, wantErr: true},
		{name: "mismatched", streams: [][]float64{{1, 2}, {1}}, wantErr: true},
		{name: "valid", streams: [][]float64{{1, 2}, {3, 4}}, wantRows: 2, wantTable: true},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given zip with "+testCase.name, testingTB, func() {
			zipStage := NewZip(datura.Acquire("zip-config", datura.APPJSON))
			artifact := zipInbound(testCase.streams)
			err := nomagique.RoundTripArtifact(artifact, zipStage)

			if testCase.wantErr {
				So(err, ShouldNotBeNil)
				return
			}

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
		config := datura.Acquire("node-ring-config", datura.APPJSON).
			Poke(4, "nodeCount").
			Poke(8, "capacity")
		pipeline := nomagique.Number(
			NewNodeRing(config),
			NewZip(datura.Acquire("zip-config", datura.APPJSON)),
		)
		artifact := datura.Acquire("node-ring-inbound", datura.APPJSON)

		for index := range 16 {
			artifact.Poke([]float64{
				float64(index) * 0.1,
				float64(index) * 0.2,
				float64(index) * 0.5,
				float64(index) * 0.05,
			}, "batch")
			err := nomagique.RoundTripArtifact(artifact, pipeline)

			So(err, ShouldBeNil)
		}

		Convey("It should retain bounded aligned table rows", func() {
			So(datura.Peek[float64](artifact, "table", "rowCount"), ShouldEqual, 8)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 8)
		})
	})

	Convey("Given partial node inputs", testingTB, func() {
		config := datura.Acquire("node-ring-config", datura.APPJSON).
			Poke(4, "nodeCount").
			Poke(8, "capacity")
		nodeRing := NewNodeRing(config)
		artifact := datura.Acquire("node-ring-inbound", datura.APPJSON).
			Poke([]float64{1}, "batch")
		err := nomagique.RoundTripArtifact(artifact, nodeRing)

		Convey("It should reject misaligned rows", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkZip_Read(testingTB *testing.B) {
	zipStage := NewZip(datura.Acquire("zip-config", datura.APPJSON))
	artifact := zipInbound([][]float64{
		{0.1, 0.2, 0.3, 0.4},
		{0.5, 0.6, 0.7, 0.8},
		{1.0, 1.1, 1.2, 1.3},
		{2.0, 2.1, 2.2, 2.3},
	})

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = nomagique.RoundTripArtifact(artifact, zipStage)
	}
}
