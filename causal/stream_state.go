package causal

import (
	"strconv"

	"github.com/theapemachine/datura"
)

type preservedStreams struct {
	nodeCount float64
	streams   map[string][]float64
}

func preserveStreams(artifact *datura.Artifact) preservedStreams {
	nodeCount := datura.Peek[float64](artifact, "streams", "nodeCount")
	streamCount := int(nodeCount)
	preserved := preservedStreams{
		nodeCount: nodeCount,
		streams:   make(map[string][]float64, streamCount),
	}

	for nodeIndex := range streamCount {
		key := strconv.Itoa(nodeIndex)
		preserved.streams[key] = datura.Peek[[]float64](artifact, "streams", key)
	}

	return preserved
}

func restoreStreams(artifact *datura.Artifact, preserved preservedStreams) {
	if preserved.nodeCount <= 0 && len(preserved.streams) == 0 {
		return
	}

	artifact.Poke(preserved.nodeCount, "streams", "nodeCount")

	for key, stream := range preserved.streams {
		artifact.Poke(stream, "streams", key)
	}
}
