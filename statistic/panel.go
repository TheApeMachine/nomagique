package statistic

import (
	"math"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/theapemachine/datura"
)

/*
Panel registers keyed samples for cross-section pipelines.
*/
type Panel struct {
	artifact *datura.Artifact
}

/*
NewPanel returns a keyed sample registry for cross-section pipelines.
*/
func NewPanel() *Panel {
	return &Panel{
		artifact: datura.Acquire("panel", datura.APPJSON),
	}
}

func (panel *Panel) Write(p []byte) (int, error) {
	return panel.artifact.Write(p)
}

func (panel *Panel) Read(p []byte) (int, error) {
	member := datura.Peek[float64](panel.artifact, "member")
	sample := datura.Peek[float64](panel.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return panel.artifact.Read(p)
	}

	if attributePresent(panel.artifact, "member") {
		peers := datura.Peek[map[string]float64](panel.artifact, "peers")

		if peers == nil {
			peers = map[string]float64{}
		}

		peers[memberKey(member)] = sample
		panel.artifact.Poke(peers, "peers")
	}

	panel.artifact.Poke(datura.Map[float64]{"value": sample}, "output")

	return panel.artifact.Read(p)
}

func (panel *Panel) Close() error {
	return nil
}

func attributePresent(artifact *datura.Artifact, key string) bool {
	raw, err := artifact.Attributes()

	if err != nil || len(raw) == 0 {
		return false
	}

	_, getErr := sonic.Get(raw, key)

	return getErr == nil
}

func memberKey(member float64) string {
	return strconv.FormatFloat(member, 'g', -1, 64)
}
