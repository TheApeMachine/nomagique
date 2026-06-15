package algorithm

import (
	"github.com/theapemachine/datura"
)

/*
NodeRing retains aligned per-node sample history for tabular causal stages.

Write with one scalar per node appends one aligned row. Capacity trims the
oldest row per node when history exceeds the configured bound.
*/
type NodeRing struct {
	artifact  *datura.Artifact
	nodeCount int
	capacity  int
	streams   [][]float64
}

/*
NewNodeRing returns a bounded multi-node history accumulator.
*/
func NewNodeRing(nodeCount int, capacity int) *NodeRing {
	if nodeCount <= 0 {
		nodeCount = 1
	}

	if capacity <= 0 {
		capacity = 1
	}

	streams := make([][]float64, nodeCount)

	for nodeIndex := range streams {
		streams[nodeIndex] = make([]float64, 0, capacity)
	}

	return &NodeRing{
		artifact:  datura.Acquire("node-ring", datura.Artifact_Type_json),
		nodeCount: nodeCount,
		capacity:  capacity,
		streams:   streams,
	}
}

func (nodeRing *NodeRing) Write(p []byte) (int, error) {
	return nodeRing.artifact.Write(p)
}

func (nodeRing *NodeRing) Read(p []byte) (int, error) {
	rehydrateArtifact(&nodeRing.artifact, "node-ring", datura.Artifact_Type_json)

	payload, err := nodeRing.artifact.Payload()

	if err == nil {
		row := payloadSamples(payload)

		if len(row) == nodeRing.nodeCount {
			for nodeIndex, sample := range row {
				stream := nodeRing.streams[nodeIndex]
				stream = append(stream, sample)

				if len(stream) > nodeRing.capacity {
					stream = stream[len(stream)-nodeRing.capacity:]
				}

				nodeRing.streams[nodeIndex] = stream
			}

			out := encodePayload(row[nodeRing.nodeCount-1])
			_ = nodeRing.artifact.SetPayload(out)
		}

		if len(row) != nodeRing.nodeCount {
			out := encodePayload(0)
			_ = nodeRing.artifact.SetPayload(out)
		}
	}

	return nodeRing.artifact.Read(p)
}

func (nodeRing *NodeRing) Close() error {
	return nil
}

/*
Streams returns the live per-node histories backing tabular evaluation.
*/
func (nodeRing *NodeRing) Streams() [][]float64 {
	if nodeRing == nil {
		return nil
	}

	return nodeRing.streams
}

/*
AlignedLength returns the shortest non-empty node history length.
*/
func (nodeRing *NodeRing) AlignedLength() int {
	if nodeRing == nil {
		return 0
	}

	return alignedStreamLength(nodeRing.streams)
}

/*
Reset clears retained node histories.
*/
func (nodeRing *NodeRing) Reset() error {
	if nodeRing == nil {
		return nil
	}

	for nodeIndex := range nodeRing.streams {
		nodeRing.streams[nodeIndex] = nodeRing.streams[nodeIndex][:0]
	}

	return nil
}

func alignedStreamLength(streams [][]float64) int {
	length := 0

	for nodeIndex := range streams {
		streamLength := len(streams[nodeIndex])

		if streamLength == 0 {
			return 0
		}

		if length == 0 || streamLength < length {
			length = streamLength
		}
	}

	return length
}
