package causal

import (
	"math"
	"strconv"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/structure"
)

/*
NodeRing retains aligned per-node sample history for tabular causal stages.

Write with one scalar per node on batch appends one aligned row. Capacity trims
the oldest sample per node when history exceeds config.capacity. Compose with
NewZip so table.* attributes materialize for downstream causal stages.
*/
type NodeRing struct {
	artifact *datura.Artifact
}

/*
NewNodeRing returns a bounded multi-node history accumulator.
*/
func NewNodeRing() *NodeRing {
	return &NodeRing{
		artifact: datura.Acquire("node-ring", datura.APPJSON),
	}
}

func (nodeRing *NodeRing) Write(p []byte) (int, error) {
	preserved := nodeRing.preserveStreams()

	n, err := nodeRing.artifact.Write(p)

	nodeRing.restoreStreams(preserved)

	return n, err
}

func (nodeRing *NodeRing) Read(p []byte) (int, error) {
	nodeCount := nodeRing.nodeCountFromArtifact()
	capacity := nodeRing.capacityFromArtifact()
	row := datura.Peek[[]float64](nodeRing.artifact, "batch")
	output := datura.Map[float64]{"value": 0}

	if len(row) == nodeCount {
		for nodeIndex, sample := range row {
			if math.IsNaN(sample) || math.IsInf(sample, 0) {
				nodeRing.artifact.Poke(output, "output")

				return nodeRing.artifact.Read(p)
			}

			stream := nodeRing.loadStream(nodeIndex, capacity)
			stream.Push(sample)
			nodeRing.persistStream(nodeIndex, stream)
		}

		nodeRing.artifact.Poke(float64(nodeCount), "streams", "nodeCount")
		output["value"] = row[nodeCount-1]
	}

	nodeRing.artifact.Poke(output, "output")

	return nodeRing.artifact.Read(p)
}

func (nodeRing *NodeRing) Close() error {
	return nil
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

func (nodeRing *NodeRing) loadStream(nodeIndex int, capacity int) *structure.ListRing[float64] {
	key := strconv.Itoa(nodeIndex)
	values := datura.Peek[[]float64](nodeRing.artifact, "streams", key)
	stream := structure.NewListRing[float64](capacity, nil)

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
	nodeCount := int(datura.Peek[float64](nodeRing.artifact, "config", "nodeCount"))

	if nodeCount <= 0 {
		nodeCount = 1
	}

	return nodeCount
}

func (nodeRing *NodeRing) capacityFromArtifact() int {
	capacity := int(datura.Peek[float64](nodeRing.artifact, "config", "capacity"))

	if capacity <= 0 {
		capacity = 1
	}

	return capacity
}

type nodeRingStreams struct {
	nodeCount float64
	streams   map[string][]float64
}

func (nodeRing *NodeRing) preserveStreams() nodeRingStreams {
	nodeCount := datura.Peek[float64](nodeRing.artifact, "streams", "nodeCount")
	streamCount := int(nodeCount)
	preserved := nodeRingStreams{
		nodeCount: nodeCount,
		streams:   make(map[string][]float64, streamCount),
	}

	for nodeIndex := range streamCount {
		key := strconv.Itoa(nodeIndex)
		preserved.streams[key] = datura.Peek[[]float64](nodeRing.artifact, "streams", key)
	}

	return preserved
}

func (nodeRing *NodeRing) restoreStreams(preserved nodeRingStreams) {
	if preserved.nodeCount <= 0 && len(preserved.streams) == 0 {
		return
	}

	nodeRing.artifact.Poke(preserved.nodeCount, "streams", "nodeCount")

	for key, stream := range preserved.streams {
		nodeRing.artifact.Poke(stream, "streams", key)
	}
}
