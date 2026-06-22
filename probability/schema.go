package probability

import "github.com/theapemachine/datura"

func wireRole(state *datura.Artifact) string {
	role := datura.Peek[string](state, "channel")

	if role == "" {
		role, _ = state.Role()
	}

	return role
}

func configString(config, state *datura.Artifact, key string) string {
	role := wireRole(state)

	if role != "" {
		scoped := datura.Peek[string](config, role, key)

		if scoped != "" {
			return scoped
		}
	}

	return datura.Peek[string](config, key)
}

func configFloat64(config, state *datura.Artifact, key string) float64 {
	role := wireRole(state)

	if role != "" && datura.KeyPresent(config, role, key) {
		return datura.Peek[float64](config, role, key)
	}

	if datura.KeyPresent(config, key) {
		return datura.Peek[float64](config, key)
	}

	return 0
}

func configStringSlice(config, state *datura.Artifact, key string) []string {
	role := wireRole(state)

	if role != "" {
		scoped := datura.Peek[[]string](config, role, key)

		if len(scoped) > 0 {
			return scoped
		}
	}

	return datura.Peek[[]string](config, key)
}
