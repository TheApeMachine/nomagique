package statistic

import (
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
keyPresent reports whether the path exists in either the artifact attributes or
its decrypted payload, mirroring datura.Peek's two-region lookup.
*/
func KeyPresent(artifact *datura.Artifact, path ...any) bool {
	if rawAttributes, err := artifact.Attributes(); err == nil && len(rawAttributes) > 0 {
		if node, getErr := sonic.Get(rawAttributes, path...); getErr == nil && node.Exists() {
			return true
		}
	}

	payload := artifact.DecryptPayload()

	if len(payload) == 0 {
		return false
	}

	node, getErr := sonic.Get(payload, path...)

	return getErr == nil && node.Exists()
}

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
WireInputKey resolves the inbound scalar key from config attributes.
*/
func WireInputKey(config, state *datura.Artifact) (string, error) {
	inputKey := ConfigString(config, state, "input")

	if inputKey == "" {
		inputKey = ConfigString(config, state, "sampleKey")
	}

	if inputKey == "" && KeyPresent(state, "sample") {
		return "sample", nil
	}

	if inputKey == "" {
		inputKeys := ConfigStringSlice(config, state, "inputs")

		if len(inputKeys) == 0 {
			inputKeys = datura.Peek[[]string](state, "inputs")
		}

		if len(inputKeys) == 1 && inputKeys[0] != "" {
			return inputKeys[0], nil
		}
	}

	if inputKey == "" {
		return "", errnie.Error(errnie.Err(
			errnie.Validation,
			"wire: config input or sampleKey required",
			nil,
		))
	}

	return inputKey, nil
}

/*
ConfigFloat64 reads a float attribute from config, preferring role-scoped keys.
*/
func ConfigFloat64(config, state *datura.Artifact, key string) float64 {
	role := WireRole(state)

	if role != "" && KeyPresent(config, role, key) {
		return datura.Peek[float64](config, role, key)
	}

	if KeyPresent(config, key) {
		return datura.Peek[float64](config, key)
	}

	return 0
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
		if KeyPresent(state, rootKey, wireKey) {
			return datura.Peek[float64](state, rootKey, wireKey), nil
		}
	}

	if KeyPresent(state, wireKey) {
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
