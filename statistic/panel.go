package statistic

import (
	"math"
	"slices"
	"strconv"

	"github.com/theapemachine/datura"
)

/*
Panel registers keyed samples for cross-section pipelines.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Panel struct {
	artifact *datura.Artifact
}

/*
NewPanel returns a keyed sample registry wired from config attributes on the artifact.
*/
func NewPanel(artifact *datura.Artifact) *Panel {
	artifact.Inspect("statistic", "panel", "NewPanel()")

	return &Panel{
		artifact: artifact,
	}
}

func (panel *Panel) Write(payload []byte) (int, error) {
	panel.artifact.WithPayload(payload)
	return len(payload), nil
}

func (panel *Panel) Read(payload []byte) (int, error) {
	state := datura.Acquire("panel-state", datura.APPJSON)
	state.Inspect("statistic", "panel", "Read()", "p")

	if _, err := state.Write(panel.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	member := datura.Peek[float64](state, "member")
	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return state.Read(payload)
	}

	key := memberKey(member)
	peerKeys := datura.Peek[[]string](panel.artifact, "peerKeys")

	if !slices.Contains(peerKeys, key) {
		peerKeys = append(peerKeys, key)
		panel.artifact.Poke(peerKeys, "peerKeys")
	}

	panel.artifact.Poke(sample, "peers", key)

	peers := map[string]float64{}

	for _, peerKey := range peerKeys {
		peers[peerKey] = datura.Peek[float64](panel.artifact, "peers", peerKey)
	}

	state.Merge("peerKeys", peerKeys)
	state.Merge("peers", peers)
	state.MergeOutput("value", sample)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (panel *Panel) Close() error {
	return nil
}

func memberKey(member float64) string {
	return strconv.FormatFloat(member, 'g', -1, 64)
}

func panelPeers(artifact *datura.Artifact) map[string]float64 {
	peerKeys := datura.Peek[[]string](artifact, "peerKeys")
	peers := map[string]float64{}

	for _, peerKey := range peerKeys {
		peers[peerKey] = datura.Peek[float64](artifact, "peers", peerKey)
	}

	return peers
}
