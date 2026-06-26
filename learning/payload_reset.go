package learning

import "github.com/theapemachine/datura"

func payloadHasReset(payload []byte) bool {
	if len(payload) == 0 {
		return false
	}

	state := datura.Acquire("learning-inbound", datura.APPJSON)

	if _, err := state.Unpack(payload); err != nil {
		return false
	}

	return datura.Peek[float64](state, "reset") > 0
}
