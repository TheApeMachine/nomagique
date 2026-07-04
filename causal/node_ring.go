package causal

import (
	"fmt"
	"math"
	"strconv"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
NodeRing retains aligned per-node sample history for tabular causal stages.

Write with one scalar per node on batch appends one aligned row. Capacity trims
the oldest sample per node when history exceeds config.capacity. Compose with
NewZip so table.* attributes materialize for downstream causal stages.
The constructor artifact holds config and stream history; Write buffers inbound wire.
*/
type NodeRing struct {
	artifact  *datura.Artifact
	nodeCount int
	capacity  int
	streams   [][]float64
}

/*
NewNodeRing returns a bounded multi-node history accumulator wired from config attributes.
*/
func NewNodeRing(artifact *datura.Artifact) *NodeRing {
	nodeCount := int(datura.Peek[float64](artifact, "nodeCount"))
	if nodeCount <= 0 {
		nodeCount = 1
	}

	capacity := int(datura.Peek[float64](artifact, "capacity"))
	if capacity <= 0 {
		capacity = 1
	}

	streams := make([][]float64, nodeCount)
	for nodeIndex := range nodeCount {
		values := datura.Peek[[]float64](artifact, "streams", strconv.Itoa(nodeIndex))
		if len(values) > capacity {
			values = values[len(values)-capacity:]
		}
		streams[nodeIndex] = append([]float64(nil), values...)
	}

	return &NodeRing{
		artifact:  artifact,
		nodeCount: nodeCount,
		capacity:  capacity,
		streams:   streams,
	}
}

func (nodeRing *NodeRing) Read(p []byte) (int, error) {
	state := datura.Acquire("node-ring-state", datura.APPJSON)

	if _, err := state.Unpack(nodeRing.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal: state write failed",
			err,
		))
	}

	row := datura.Peek[[]float64](state, "batch")

	if len(row) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal node-ring: batch required",
			nil,
		))
	}

	if len(row) != nodeRing.nodeCount {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("causal node-ring: batch length %d does not match nodeCount %d", len(row), nodeRing.nodeCount),
			nil,
		))
	}

	output := 0.0

	for nodeIndex, sample := range row {
		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"causal node-ring: sample is non-finite",
				fmt.Errorf("causal: node %d sample is non-finite", nodeIndex),
			))
		}

		nodeRing.streams[nodeIndex] = append(nodeRing.streams[nodeIndex], sample)
		if len(nodeRing.streams[nodeIndex]) > nodeRing.capacity {
			copy(nodeRing.streams[nodeIndex], nodeRing.streams[nodeIndex][len(nodeRing.streams[nodeIndex])-nodeRing.capacity:])
			nodeRing.streams[nodeIndex] = nodeRing.streams[nodeIndex][:nodeRing.capacity]
		}
	}

	output = row[nodeRing.nodeCount-1]

	nodeRing.copyStreamsTo(state)
	state.MergeOutput("value", output)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(p)
}

func (nodeRing *NodeRing) Write(p []byte) (int, error) {
	nodeRing.artifact.WithPayload(p)
	return len(p), nil
}

func (nodeRing *NodeRing) Close() error {
	return nil
}

/*
CopyStreamsTo copies retained node streams onto state for downstream zip stages.
*/
func (nodeRing *NodeRing) CopyStreamsTo(state *datura.Artifact) {
	nodeRing.copyStreamsTo(state)
}

/*
AlignedLength returns the shortest non-empty node history length.
*/
func (nodeRing *NodeRing) AlignedLength() int {
	if nodeRing == nil {
		return 0
	}

	if nodeRing.nodeCount <= 0 {
		return 0
	}

	length := 0

	for nodeIndex := range nodeRing.nodeCount {
		streamLength := len(nodeRing.streams[nodeIndex])
		if streamLength == 0 {
			return 0
		}

		if length == 0 || streamLength < length {
			length = streamLength
		}
	}

	return length
}

func (nodeRing *NodeRing) copyStreamsTo(state *datura.Artifact) {
	if nodeRing.nodeCount <= 0 {
		return
	}

	state.Poke(float64(nodeRing.nodeCount), "streams", "nodeCount")

	for nodeIndex := range nodeRing.nodeCount {
		state.Poke(nodeRing.streams[nodeIndex], "streams", strconv.Itoa(nodeIndex))
	}
}
