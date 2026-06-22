package statistic

import (
	"fmt"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
WireRole resolves the inbound channel or artifact role for scoped config lookup.
*/
func WireRole(state *datura.Artifact) string {
	role := datura.Peek[string](state, "channel")

	if role == "" {
		role, _ = state.Role()
	}

	return role
}

/*
ConfigString reads a string attribute from config, preferring role-scoped keys.
*/
func ConfigString(config, state *datura.Artifact, key string) string {
	role := WireRole(state)

	if role != "" {
		scoped := datura.Peek[string](config, role, key)

		if scoped != "" {
			return scoped
		}
	}

	return datura.Peek[string](config, key)
}

/*
ConfigStringSlice reads a string slice attribute from config, preferring role-scoped keys.
*/
func ConfigStringSlice(config, state *datura.Artifact, key string) []string {
	role := WireRole(state)

	if role != "" {
		scoped := datura.Peek[[]string](config, role, key)

		if len(scoped) > 0 {
			return scoped
		}
	}

	return datura.Peek[[]string](config, key)
}

/*
WireInputKey resolves the inbound scalar key from config with a package default.
*/
func WireInputKey(config, state *datura.Artifact, defaultKey string) string {
	inputKey := ConfigString(config, state, "input")

	if inputKey == "" {
		inputKey = ConfigString(config, state, "sampleKey")
	}

	if inputKey == "" {
		inputKey = defaultKey
	}

	return inputKey
}

/*
WireScalar resolves one inbound scalar using config root/inputs and pipeline state.
*/
func WireScalar(config, state *datura.Artifact, wireKey string) (float64, error) {
	rootKey := ConfigString(config, state, "root")

	if rootKey == "" {
		rootKey = datura.Peek[string](state, "root")
	}

	return WireScalarAt(config, state, rootKey, wireKey)
}

/*
WireScalarAt resolves one scalar from an explicit root namespace on state.
*/
func WireScalarAt(
	config, state *datura.Artifact,
	rootKey, wireKey string,
) (float64, error) {
	if wireKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"wire: empty wire key",
			nil,
		))
	}

	inputKeys := ConfigStringSlice(config, state, "inputs")

	if len(inputKeys) == 0 {
		inputKeys = datura.Peek[[]string](state, "inputs")
	}

	if rootKey == "features" && len(inputKeys) > 0 && InputIndex(inputKeys, wireKey) >= 0 {
		return FeatureColumn(state, wireKey)
	}

	if rootKey != "" && rootKey != "features" {
		if datura.KeyPresent(state, rootKey, wireKey) {
			return datura.Peek[float64](state, rootKey, wireKey), nil
		}
	}

	if datura.KeyPresent(state, wireKey) {
		return datura.Peek[float64](state, wireKey), nil
	}

	return 0, errnie.Error(errnie.Err(
		errnie.Validation,
		fmt.Sprintf("wire: key %q not found", wireKey),
		nil,
	))
}

/*
InputIndex returns the index of wireKey in inputKeys, or -1 when absent.
*/
func InputIndex(inputKeys []string, wireKey string) int {
	for index, inputKey := range inputKeys {
		if inputKey == wireKey {
			return index
		}
	}

	return -1
}
