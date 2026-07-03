package causal

import (
	"fmt"
	"math"
	"strconv"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/structure"
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
	artifact *datura.Artifact
}

/*
NewNodeRing returns a bounded multi-node history accumulator wired from config attributes.
*/
func NewNodeRing(artifact *datura.Artifact) *NodeRing {
	return &NodeRing{
		artifact: artifact,
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

	nodeCount := nodeRing.nodeCountFromArtifact()
	capacity := nodeRing.capacityFromArtifact()
	row := datura.Peek[[]float64](state, "batch")

	if len(row) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"causal node-ring: batch required",
			nil,
		))
	}

	if len(row) != nodeCount {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("causal node-ring: batch length %d does not match nodeCount %d", len(row), nodeCount),
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

		stream := nodeRing.loadStream(nodeIndex, capacity)
		stream.Push(sample)
		nodeRing.persistStream(nodeIndex, stream)
	}

	nodeRing.artifact.Poke(float64(nodeCount), "streams", "nodeCount")
	output = row[nodeCount-1]

	nodeRing.copyStreamsTo(state)
	state.MergeOutput("value", output)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(p)
}

func (nodeRing *NodeRing) Write(p []byte) (int, error) {
	preserved := preserveStreams(nodeRing.artifact)
	nodeRing.artifact.WithPayload(p)
	restoreStreams(nodeRing.artifact, preserved)
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

	nodeCount := int(datura.Peek[float64](nodeRing.artifact, "streams", "nodeCount"))

	if nodeCount <= 0 {
		return 0
	}

	length := 0

	for nodeIndex := range nodeCount {
		streamLength := len(datura.Peek[[]float64](nodeRing.artifact, "streams", strconv.Itoa(nodeIndex)))

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
	nodeCount := int(datura.Peek[float64](nodeRing.artifact, "streams", "nodeCount"))

	if nodeCount <= 0 {
		return
	}

	state.Poke(float64(nodeCount), "streams", "nodeCount")

	for nodeIndex := range nodeCount {
		key := strconv.Itoa(nodeIndex)
		state.Poke(datura.Peek[[]float64](nodeRing.artifact, "streams", key), "streams", key)
	}
}

func (nodeRing *NodeRing) loadStream(nodeIndex int, capacity int) *structure.ListRing[float64] {
	key := strconv.Itoa(nodeIndex)
	values := datura.Peek[[]float64](nodeRing.artifact, "streams", key)
	stream := structure.NewListRing[float64](capacity)

	for _, value := range values {
		stream.Push(value)
	}

	return stream
}

func (nodeRing *NodeRing) persistStream(nodeIndex int, stream *structure.ListRing[float64]) {
	values := make([]float64, 0, stream.Len())

	stream.Do(func(value float64) {
		values = append(values, value)
	})

	nodeRing.artifact.Poke(values, "streams", strconv.Itoa(nodeIndex))
}

func (nodeRing *NodeRing) nodeCountFromArtifact() int {
	nodeCount := int(datura.Peek[float64](nodeRing.artifact, "nodeCount"))

	if nodeCount <= 0 {
		nodeCount = 1
	}

	return nodeCount
}

func (nodeRing *NodeRing) capacityFromArtifact() int {
	capacity := int(datura.Peek[float64](nodeRing.artifact, "capacity"))

	if capacity <= 0 {
		capacity = 1
	}

	return capacity
}
