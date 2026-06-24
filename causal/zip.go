package causal

import (
	"errors"
	"strconv"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Zip transposes aligned per-node streams on the artifact payload into table.* rows
for downstream tabular causal stages. The constructor artifact retains stream config;
Write buffers inbound wire on its payload.
*/
type Zip struct {
	artifact *datura.Artifact
}

/*
NewZip returns a stream-to-table zip stage wired from config attributes on the artifact.
*/
func NewZip(artifact *datura.Artifact) *Zip {
	return &Zip{
		artifact: artifact,
	}
}

func (zipStage *Zip) Read(p []byte) (int, error) {
	state := datura.Acquire("zip-state", datura.APPJSON)

	if _, err := state.Write(zipStage.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	state.Inspect("causal", "zip", "Read()", "p")

	if rows, tableOK := tableRows(state); tableOK {
		state.MergeOutput("value", float64(len(rows)))
		state.Poke("output", "root")
		state.Poke([]string{"value"}, "inputs")
		return state.Read(p)
	}

	streams, ok := zipStage.streamsFromPayload(state)

	if !ok {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal zip: missing node streams",
			errors.New("causal: node streams missing"),
		))
	}

	rows, rowsOK := zipStage.transpose(streams)

	if !rowsOK {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal zip: stream transpose failed",
			errors.New("causal: aligned stream transpose failed"),
		))
	}

	rowCount := len(rows)
	nodeCount := len(rows[0])
	flat := make([]float64, 0, rowCount*nodeCount)

	for rowIndex := range rows {
		flat = append(flat, rows[rowIndex]...)
	}

	state.Poke(float64(rowCount), "table", "rowCount")
	state.Poke(float64(nodeCount), "table", "nodeCount")
	state.Poke(flat, "table", "rows")
	state.MergeOutput("value", float64(rowCount))
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")
	return state.Read(p)
}

func (zipStage *Zip) Write(p []byte) (int, error) {
	preserved := zipStage.preserveStreams()
	zipStage.artifact.WithPayload(p)

	if datura.Peek[float64](zipStage.artifact, "streams", "nodeCount") <= 0 {
		zipStage.restoreStreams(preserved)
	}

	return len(p), nil
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

func (zipStage *Zip) streamsFromPayload(state *datura.Artifact) ([][]float64, bool) {
	nodeCount := int(datura.Peek[float64](state, "streams", "nodeCount"))

	if nodeCount <= 0 {
		return nil, false
	}

	streams := make([][]float64, nodeCount)

	for nodeIndex := range nodeCount {
		streams[nodeIndex] = datura.Peek[[]float64](
			state,
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
