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

	if _, err := state.Unpack(zipStage.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: state write failed",
			err,
		))
	}

	if datura.Peek[float64](state, "table", "rowCount") > 0 {
		rows, tableErr := tableRows(state)

		if tableErr != nil {
			return 0, tableErr
		}

		state.MergeOutput("value", float64(len(rows)))
		state.Poke("output", "root")
		state.Poke([]string{"value"}, "inputs")

		return state.PackInto(p)
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
	return state.PackInto(p)
}

func (zipStage *Zip) Write(p []byte) (int, error) {
	preserved := preserveStreams(zipStage.artifact)
	zipStage.artifact.WithPayload(p)

	if datura.Peek[float64](zipStage.artifact, "streams", "nodeCount") <= 0 {
		restoreStreams(zipStage.artifact, preserved)
	}

	return len(p), nil
}

func (zipStage *Zip) Close() error {
	return nil
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
