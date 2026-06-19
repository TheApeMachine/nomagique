package causal

import (
	"strconv"

	"github.com/theapemachine/datura"
)

/*
Zip transposes aligned per-node streams on the artifact payload into table.* rows
for downstream tabular causal stages.
*/
type Zip struct {
	artifact *datura.Artifact
}

/*
NewZip returns a stream-to-table zip stage.
*/
func NewZip() *Zip {
	return &Zip{
		artifact: datura.Acquire("zip", datura.APPJSON),
	}
}

func (zipStage *Zip) Write(p []byte) (int, error) {
	preserved := zipStage.preserveStreams()

	n, err := zipStage.artifact.Write(p)

	if datura.Peek[float64](zipStage.artifact, "streams", "nodeCount") <= 0 {
		zipStage.restoreStreams(preserved)
	}

	return n, err
}

func (zipStage *Zip) Read(p []byte) (int, error) {
	streams, ok := zipStage.streamsFromPayload()

	if !ok {
		zipStage.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return zipStage.artifact.Read(p)
	}

	rows, rowsOK := zipStage.transpose(streams)

	if !rowsOK {
		zipStage.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return zipStage.artifact.Read(p)
	}

	rowCount := len(rows)
	nodeCount := len(rows[0])
	flat := make([]float64, 0, rowCount*nodeCount)

	for rowIndex := range rows {
		flat = append(flat, rows[rowIndex]...)
	}

	zipStage.artifact.
		Poke(float64(rowCount), "table", "rowCount").
		Poke(float64(nodeCount), "table", "nodeCount").
		Poke(flat, "table", "rows")
	zipStage.artifact.Poke(datura.Map[float64]{"value": float64(rowCount)}, "output")

	return zipStage.artifact.Read(p)
}

func (zipStage *Zip) Close() error {
	return nil
}

type preservedStreams struct {
	nodeCount float64
	streams   map[string][]float64
}

func (zipStage *Zip) preserveStreams() preservedStreams {
	nodeCount := datura.Peek[float64](zipStage.artifact, "streams", "nodeCount")
	streamCount := int(nodeCount)
	preserved := preservedStreams{
		nodeCount: nodeCount,
		streams:   make(map[string][]float64, streamCount),
	}

	for nodeIndex := range streamCount {
		key := strconv.Itoa(nodeIndex)
		preserved.streams[key] = datura.Peek[[]float64](zipStage.artifact, "streams", key)
	}

	return preserved
}

func (zipStage *Zip) restoreStreams(preserved preservedStreams) {
	if preserved.nodeCount <= 0 && len(preserved.streams) == 0 {
		return
	}

	zipStage.artifact.Poke(preserved.nodeCount, "streams", "nodeCount")

	for key, stream := range preserved.streams {
		zipStage.artifact.Poke(stream, "streams", key)
	}
}

func (zipStage *Zip) streamsFromPayload() ([][]float64, bool) {
	nodeCount := int(datura.Peek[float64](zipStage.artifact, "streams", "nodeCount"))

	if nodeCount <= 0 {
		return nil, false
	}

	streams := make([][]float64, nodeCount)

	for nodeIndex := range nodeCount {
		streams[nodeIndex] = datura.Peek[[]float64](
			zipStage.artifact,
			"streams",
			strconv.Itoa(nodeIndex),
		)
	}

	return streams, true
}

func (zipStage *Zip) transpose(streams [][]float64) ([][]float64, bool) {
	if len(streams) == 0 {
		return nil, false
	}

	length := len(streams[0])

	if length == 0 {
		return nil, false
	}

	for _, stream := range streams {
		if len(stream) != length {
			return nil, false
		}
	}

	rows := make([][]float64, length)

	for rowIndex := range rows {
		rows[rowIndex] = make([]float64, len(streams))

		for nodeIndex, stream := range streams {
			rows[rowIndex][nodeIndex] = stream[rowIndex]
		}
	}

	return rows, true
}
